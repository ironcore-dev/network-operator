// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
)

// GetOptions holds the configuration for the "get" subcommand.
type GetOptions struct {
	Root       *RootOptions
	Resource   ResourceDef
	Labels     LabelFlags
	PrintFlags *genericclioptions.PrintFlags

	Namespace string
}

// NewCmdGet returns the "get" command with a subcommand for each known resource type.
func NewCmdGet(root *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Get network-operator resources",
		SilenceUsage: true,
	}

	for _, res := range resourceDefs {
		cmd.AddCommand(newGetResourceCmd(root, res))
	}

	return cmd
}

// newGetResourceCmd returns a command that fetches and prints a specific resource type.
func newGetResourceCmd(root *RootOptions, res ResourceDef) *cobra.Command {
	o := &GetOptions{
		Root:       root,
		Resource:   res,
		PrintFlags: genericclioptions.NewPrintFlags(""),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAME...]", res.Name),
		Short: fmt.Sprintf("Get %s", res.Name),
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			return o.Run(args)
		},
	}
	cmd.Aliases = append(cmd.Aliases, res.Aliases...)

	o.Labels.AddCommonFlags(cmd)
	switch res.Kind {
	case "Interface":
		o.Labels.AddInterfaceFlags(cmd)
	case "VLAN":
		o.Labels.AddVLANFlags(cmd)
	}
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

// Complete resolves the target namespace from the kubeconfig.
func (o *GetOptions) Complete() error {
	namespace, _, err := o.Root.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// Run fetches the requested resources and prints them to the configured output.
func (o *GetOptions) Run(args []string) error {
	f := strings.ToLower(*o.PrintFlags.OutputFormat)

	var transforms []resource.RequestTransform
	if f == "" || f == "wide" {
		transforms = append(transforms, func(req *rest.Request) {
			req.SetHeader("Accept", fmt.Sprintf("application/json;as=Table;v=%s;g=%s,application/json", metav1.SchemeGroupVersion.Version, metav1.GroupName))
		})
	}

	result, err := buildResourceResult(o.Root.ConfigFlags, o.Namespace, o.Root.AllNamespaces, o.Resource.QualifiedName(), args, o.Labels.BuildSelector(), transforms...)
	if err != nil {
		return err
	}

	infos, err := result.Infos()
	if err != nil {
		return err
	}

	// Table output (default or -o wide).
	if f == "" || f == "wide" {
		if len(infos) == 0 {
			fmt.Fprintf(o.Root.IOStreams.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
			return nil
		}
		printer := printers.NewTablePrinter(printers.PrintOptions{
			Wide:          f == "wide",
			WithNamespace: o.Root.AllNamespaces,
		})
		for _, info := range infos {
			if info.Object == nil {
				continue
			}
			table := &metav1.Table{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, table); err != nil {
				return err
			}
			if err := printer.PrintObj(table, o.Root.IOStreams.Out); err != nil {
				return err
			}
		}
		return nil
	}

	for _, info := range infos {
		if info.Object != nil && info.Object.GetObjectKind().GroupVersionKind().Empty() {
			info.Object.GetObjectKind().SetGroupVersionKind(info.Mapping.GroupVersionKind)
		}
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	// -o name prints each resource individually.
	if f == "name" {
		for _, info := range infos {
			if info.Object != nil {
				if err := printer.PrintObj(info.Object, o.Root.IOStreams.Out); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Structured output (-o json, -o yaml, etc.).
	// A single resource is printed directly; multiple resources are wrapped in a List.
	if len(infos) == 1 && infos[0].Object != nil {
		return printer.PrintObj(infos[0].Object, o.Root.IOStreams.Out)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "List"})
	list.SetResourceVersion("")
	for _, info := range infos {
		if info.Object != nil {
			list.Items = append(list.Items, *info.Object.(*unstructured.Unstructured))
		}
	}
	return printer.PrintObj(list, o.Root.IOStreams.Out)
}

// buildResourceResult constructs a resource.Result for the given resource type,
// optional names, and label selector using the Kubernetes resource builder.
func buildResourceResult(configFlags *genericclioptions.ConfigFlags, namespace string, allNamespaces bool, resourceName string, args []string, labelSelector string, transforms ...resource.RequestTransform) (*resource.Result, error) {
	resourceArgs := append([]string{resourceName}, args...)

	builder := resource.NewBuilder(configFlags).
		Unstructured().
		NamespaceParam(namespace).
		DefaultNamespace().
		AllNamespaces(allNamespaces).
		ResourceTypeOrNameArgs(true, resourceArgs...).
		LabelSelectorParam(labelSelector).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(transforms...)

	result := builder.Do()
	if err := result.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
