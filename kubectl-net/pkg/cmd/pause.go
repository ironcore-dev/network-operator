// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// PauseOptions holds the configuration for the "pause" and "unpause" subcommands.
type PauseOptions struct {
	Root       *RootOptions
	Resource   ResourceDef
	Labels     LabelFlags
	PrintFlags *genericclioptions.PrintFlags

	Pause     bool
	Recursive bool
	Namespace string
}

// NewCmdPause returns a "pause" or "unpause" command depending on the pause flag,
// with a subcommand for each known resource type.
func NewCmdPause(root *RootOptions, pause bool) *cobra.Command {
	use := "pause"
	action := "paused"
	short := "Pause network-operator resources"
	if !pause {
		use = "unpause"
		action = "unpaused"
		short = "Unpause network-operator resources"
	}

	cmd := &cobra.Command{
		Use:          use,
		Short:        short,
		SilenceUsage: true,
	}

	for _, res := range resourceDefs {
		cmd.AddCommand(newPauseResourceCmd(root, res, pause, action))
	}

	return cmd
}

// newPauseResourceCmd returns a command that pauses or unpauses a specific resource type.
func newPauseResourceCmd(root *RootOptions, res ResourceDef, pause bool, action string) *cobra.Command {
	o := &PauseOptions{
		Root:       root,
		Resource:   res,
		Pause:      pause,
		PrintFlags: genericclioptions.NewPrintFlags(action),
	}

	title := action
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [NAME...]", res.Name),
		Short: fmt.Sprintf("%s %s", title, res.Name),
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
	case "Device":
		o.Labels.AddDeviceFlags(cmd)
	case "Interface":
		o.Labels.AddInterfaceFlags(cmd)
	case "VLAN":
		o.Labels.AddVLANFlags(cmd)
	}
	if res.Kind == "Device" {
		cmd.Flags().BoolVar(&o.Recursive, "recursive", false, "Also set spec.paused on the Device to pause or unpause child resources")
	}
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

// Complete resolves the target namespace from the kubeconfig.
func (o *PauseOptions) Complete() error {
	namespace, _, err := o.Root.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// Run patches the matched resources to set or remove the paused annotation.
func (o *PauseOptions) Run(args []string) error {
	result, err := buildResourceResult(o.Root.ConfigFlags, o.Namespace, o.Root.AllNamespaces, o.Resource.QualifiedName(), args, o.Labels.BuildSelector())
	if err != nil {
		return err
	}

	infos, err := result.Infos()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return nil
	}

	restConfig, err := o.Root.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	patchBytes, err := o.buildPatch()
	if err != nil {
		return err
	}

	patched := make([]*unstructured.Unstructured, 0, len(infos))
	for _, info := range infos {
		resourceClient := dyn.Resource(info.Mapping.Resource)
		var client dynamic.ResourceInterface = resourceClient
		if info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace && info.Namespace != "" {
			client = resourceClient.Namespace(info.Namespace)
		}
		obj, err := client.Patch(context.Background(), info.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
		patched = append(patched, obj)
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	if len(patched) == 1 {
		return printer.PrintObj(patched[0], o.Root.IOStreams.Out)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "List"})
	for _, obj := range patched {
		list.Items = append(list.Items, *obj)
	}

	return printer.PrintObj(list, o.Root.IOStreams.Out)
}

// buildPatch returns the JSON merge-patch payload for setting or removing the
// paused annotation, optionally including spec.paused for recursive Device operations.
func (o *PauseOptions) buildPatch() ([]byte, error) {
	annotations := map[string]any{}
	if o.Pause {
		annotations[v1alpha1.PausedAnnotation] = "true"
	} else {
		annotations[v1alpha1.PausedAnnotation] = nil
	}

	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}

	if o.Recursive && o.Resource.Kind == "Device" {
		patch["spec"] = map[string]any{
			"paused": o.Pause,
		}
	}

	return json.Marshal(patch)
}
