// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
)

// MoveOptions holds the configuration for the "move" subcommand.
type MoveOptions struct {
	Root *RootOptions

	ToKubeconfig        string
	ToContext           string
	DryRun              bool
	Namespace           string
	ScaleDownDeployment string
	DeploymentNamespace string
}

// NewCmdMove returns the "move" command for migrating resources between clusters.
func NewCmdMove(root *RootOptions) *cobra.Command {
	o := &MoveOptions{Root: root}

	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move network-operator resources to a target cluster",
		Long: `Move network-operator resources from the source cluster to a target cluster.

All network-operator resources in the namespace are moved. The target cluster
must already have network-operator CRDs installed.`,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			return o.Run(c.Context())
		},
	}

	cmd.Flags().StringVar(&o.ToKubeconfig, "to-kubeconfig", "", "Path to the target cluster kubeconfig (required)")
	cmd.Flags().StringVar(&o.ToContext, "to-context", "", "Context in the target kubeconfig")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Print what would be moved without executing")
	cmd.Flags().StringVar(&o.ScaleDownDeployment, "scale-down-deployment", "", "Deployment to scale to 0 on source before moving")
	cmd.Flags().StringVar(&o.DeploymentNamespace, "deployment-namespace", "", "Namespace of the deployment (defaults to resource namespace)")

	_ = cmd.MarkFlagRequired("to-kubeconfig")

	return cmd
}

func (o *MoveOptions) Complete() error {
	namespace, _, err := o.Root.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// Run executes the move operation:
// preflight → discover → pause → create on target → delete from source → unpause.
func (o *MoveOptions) Run(ctx context.Context) error {
	srcConfig, err := o.Root.ConfigFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("source cluster: %w", err)
	}
	srcDyn, err := dynamic.NewForConfig(srcConfig)
	if err != nil {
		return fmt.Errorf("source cluster: %w", err)
	}

	tgtFlags := genericclioptions.NewConfigFlags(true)
	tgtFlags.KubeConfig = &o.ToKubeconfig
	if o.ToContext != "" {
		tgtFlags.Context = &o.ToContext
	}
	tgtConfig, err := tgtFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("target cluster: %w", err)
	}
	tgtDyn, err := dynamic.NewForConfig(tgtConfig)
	if err != nil {
		return fmt.Errorf("target cluster: %w", err)
	}

	// Pre-flight: verify CRDs exist on target.
	fmt.Fprintf(o.Root.IOStreams.Out, "Checking target cluster CRDs...\n")
	crd := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	for _, def := range resourceDefs {
		name := def.Name + "." + def.Group
		_, err := tgtDyn.Resource(crd).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("target cluster missing CRD: %s", name)
		}
	}

	// Ensure namespace exists on target.
	ns := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	_, err = tgtDyn.Resource(ns).Get(ctx, o.Namespace, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("checking namespace on target: %w", err)
		}
		n := &unstructured.Unstructured{}
		n.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"})
		n.SetName(o.Namespace)
		_, err = tgtDyn.Resource(ns).Create(ctx, n, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating namespace on target: %w", err)
		}
	}

	// Discover all network-operator resources in the source namespace.
	fmt.Fprintf(o.Root.IOStreams.Out, "Discovering resources in namespace %q...\n", o.Namespace)
	objects, err := o.DiscoverResources(ctx, srcDyn)
	if err != nil {
		return err
	}
	if len(objects) == 0 {
		fmt.Fprintf(o.Root.IOStreams.Out, "No resources found.\n")
		return nil
	}

	// Check nothing is already paused.
	for _, obj := range objects {
		if obj.GetKind() == "Device" {
			paused, _, _ := unstructured.NestedBool(obj.Object, "spec", "paused")
			if paused {
				return fmt.Errorf("device %q is already paused; unpause before moving", obj.GetName())
			}
		}
	}

	if o.DryRun {
		for _, obj := range objects {
			fmt.Fprintf(o.Root.IOStreams.Out, "  %s/%s\n", obj.GetKind(), obj.GetName())
		}
		fmt.Fprintf(o.Root.IOStreams.Out, "\nTotal: %d resources would be moved.\n", len(objects))
		return nil
	}

	// Scale down operator deployment on source (if requested).
	if o.ScaleDownDeployment != "" {
		ns := o.DeploymentNamespace
		if ns == "" {
			ns = o.Namespace
		}
		fmt.Fprintf(o.Root.IOStreams.Out, "Scaling down %s/%s on source...\n", ns, o.ScaleDownDeployment)
		deploy := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		patch := must(json.Marshal(map[string]any{
			"spec": map[string]any{"replicas": 0},
		}))
		_, err := srcDyn.Resource(deploy).Namespace(ns).Patch(ctx, o.ScaleDownDeployment, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("scaling down deployment %q: %w", o.ScaleDownDeployment, err)
		}
	}

	// Pause devices on source.
	fmt.Fprintf(o.Root.IOStreams.Out, "Pausing Devices on source...\n")
	device := schema.GroupVersionResource{Group: "networking.metal.ironcore.dev", Version: "v1alpha1", Resource: "devices"}
	patch := must(json.Marshal(map[string]any{
		"spec": map[string]any{"paused": true},
	}))
	for _, obj := range objects {
		if obj.GetKind() == "Device" {
			_, err := srcDyn.Resource(device).Namespace(o.Namespace).Patch(ctx, obj.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("pausing device %q: %w", obj.GetName(), err)
			}
		}
	}

	// Create on target (sorted by priority: devices, pools, allocations, claims, rest).
	slices.SortFunc(objects, func(a, b *unstructured.Unstructured) int {
		return cmp.Compare(movePriority(a), movePriority(b))
	})
	fmt.Fprintf(o.Root.IOStreams.Out, "Creating resources on target...\n")
	var errs []error
	for _, obj := range objects {
		if err := o.CreateOnTarget(ctx, tgtDyn, obj); err != nil {
			errs = append(errs, fmt.Errorf("create %s/%s: %w", obj.GetKind(), obj.GetName(), err))
		}
	}
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.Root.IOStreams.ErrOut, "  error: %v\n", err)
		}
		return fmt.Errorf("%d errors creating resources on target", len(errs))
	}

	// Delete from source.
	fmt.Fprintf(o.Root.IOStreams.Out, "Deleting resources from source...\n")
	for _, obj := range objects {
		if err := o.DeleteFromSource(ctx, srcDyn, obj); err != nil {
			errs = append(errs, fmt.Errorf("delete %s/%s: %w", obj.GetKind(), obj.GetName(), err))
		}
	}
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.Root.IOStreams.ErrOut, "  error: %v\n", err)
		}
		return fmt.Errorf("%d errors deleting resources from source", len(errs))
	}

	// Unpause devices on target.
	fmt.Fprintf(o.Root.IOStreams.Out, "Unpausing Devices on target...\n")
	patch = must(json.Marshal(map[string]any{
		"spec": map[string]any{"paused": false},
	}))
	for _, obj := range objects {
		if obj.GetKind() == "Device" {
			_, err := tgtDyn.Resource(device).Namespace(o.Namespace).Patch(ctx, obj.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("unpausing device %q: %w", obj.GetName(), err)
			}
		}
	}

	fmt.Fprintf(o.Root.IOStreams.Out, "Move complete.\n")
	return nil
}

// DiscoverResources lists all network-operator resources plus Secrets and
// ConfigMaps in the source namespace.
func (o *MoveOptions) DiscoverResources(ctx context.Context, dyn dynamic.Interface) ([]*unstructured.Unstructured, error) {
	var all []*unstructured.Unstructured
	for _, def := range resourceDefs {
		gvr := schema.GroupVersionResource{Group: def.Group, Version: def.Version, Resource: def.Name}
		list, err := dyn.Resource(gvr).Namespace(o.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return nil, fmt.Errorf("listing %s: %w", def.Name, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			item.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   def.Group,
				Version: def.Version,
				Kind:    def.Kind,
			})
			all = append(all, item)
		}
	}

	// Also move Secrets and ConfigMaps referenced by controllers.
	for _, extra := range []schema.GroupVersionResource{
		{Version: "v1", Resource: "secrets"},
		{Version: "v1", Resource: "configmaps"},
	} {
		list, err := dyn.Resource(extra).Namespace(o.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", extra.Resource, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			if item.GetName() == "kube-root-ca.crt" {
				continue
			}
			item.SetGroupVersionKind(schema.GroupVersionKind{
				Version: "v1",
				Kind:    list.Items[i].GetKind(),
			})
			all = append(all, item)
		}
	}

	return all, nil
}

// CreateOnTarget creates a single object on the target cluster.
// Owner references are stripped — controllers re-establish them on first reconcile.
func (o *MoveOptions) CreateOnTarget(ctx context.Context, dyn dynamic.Interface, obj *unstructured.Unstructured) error {
	tgt := obj.DeepCopy()

	tgt.SetResourceVersion("")
	tgt.SetUID("")
	tgt.SetCreationTimestamp(metav1.Time{})
	tgt.SetGeneration(0)
	tgt.SetManagedFields(nil)
	tgt.SetOwnerReferences(nil)
	delete(tgt.Object, "status")

	gvr := gvrFromObject(obj)
	client := dyn.Resource(gvr).Namespace(o.Namespace)
	_, err := client.Create(ctx, tgt, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		existing, err := client.Get(ctx, tgt.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		tgt.SetResourceVersion(existing.GetResourceVersion())
		_, err = client.Update(ctx, tgt, metav1.UpdateOptions{})
		return err
	}
	return nil
}

// DeleteFromSource deletes a single object from the source cluster.
func (o *MoveOptions) DeleteFromSource(ctx context.Context, dyn dynamic.Interface, obj *unstructured.Unstructured) error {
	gvr := gvrFromObject(obj)
	client := dyn.Resource(gvr).Namespace(o.Namespace)
	err := client.Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Strip finalizers to allow immediate GC (controllers are paused).
	if len(obj.GetFinalizers()) > 0 {
		patch := must(json.Marshal(map[string]any{
			"metadata": map[string]any{"finalizers": []string{}},
		}))
		_, err = client.Patch(ctx, obj.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func gvrFromObject(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	for _, def := range resourceDefs {
		if def.Kind == gvk.Kind && def.Group == gvk.Group {
			return schema.GroupVersionResource{
				Group:    def.Group,
				Version:  def.Version,
				Resource: def.Name,
			}
		}
	}
	// Fallback for core types (Secret, ConfigMap, etc.).
	switch gvk.Kind {
	case "Secret":
		return schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	case "ConfigMap":
		return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	}
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: fmt.Sprintf("%ss", gvk.Kind),
	}
}

func movePriority(obj *unstructured.Unstructured) int {
	switch obj.GetKind() {
	case "Device":
		return 0
	case "IndexPool", "IPAddressPool", "IPPrefixPool":
		return 1
	case "Index", "IPAddress", "IPPrefix":
		return 2
	case "Claim":
		return 3
	default:
		return 4
	}
}
