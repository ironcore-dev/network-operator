// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
)

// vclog is for logging in this package.
var vclog = logf.Log.WithName("networkvirtualizationedgeconfig-resource")

// SetupNetworkVirtualizationEdgeConfigWebhookWithManager registers the webhook for NetworkVirtualizationEdge in the manager.
func SetupNetworkVirtualizationEdgeConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.NetworkVirtualizationEdgeConfig{}).
		WithValidator(&NetworkVirtualizationEdgeConfigCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-nx-cisco-networking-metal-ironcore-dev-v1alpha1-networkvirtualizationedgeconfig,mutating=false,failurePolicy=Fail,sideEffects=None,groups=nx.cisco.networking.metal.ironcore.dev,resources=networkvirtualizationedgeconfigs,verbs=create;update,versions=v1alpha1,name=networkvirtualizationedgeconfig-cisco-nx-v1alpha1.kb.io,admissionReviewVersions=v1

// NetworkVirtualizationEdgeConfigCustomValidator struct is responsible for validating the NetworkVirtualizationEdgeConfig resource
// when it is created, updated, or deleted.
type NetworkVirtualizationEdgeConfigCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &NetworkVirtualizationEdgeConfigCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type NetworkVirtualizationEdgeConfig.
func (v *NetworkVirtualizationEdgeConfigCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	vc, ok := obj.(*v1alpha1.NetworkVirtualizationEdgeConfig)
	if !ok {
		return nil, fmt.Errorf("expected a NetworkVirtualizationEdgeConfig object but got %T", obj)
	}
	vclog.Info("Validation for NetworkVirtualizationEdgeConfig upon creation", "name", vc.GetName())

	return nil, validateNetworkVirtualizationEdgeConfigSpec(vc)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type NetworkVirtualizationEdgeConfig.
func (v *NetworkVirtualizationEdgeConfigCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	vc, ok := newObj.(*v1alpha1.NetworkVirtualizationEdgeConfig)

	if !ok {
		return nil, fmt.Errorf("expected a NetworkVirtualizationEdgeConfig object for the newObj but got %T", newObj)
	}
	vclog.Info("Validation for NetworkVirtualizationEdgeConfig upon update", "name", vc.GetName())

	return nil, validateNetworkVirtualizationEdgeConfigSpec(vc)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type NetworkVirtualizationEdgeConfig.
func (v *NetworkVirtualizationEdgeConfigCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*v1alpha1.NetworkVirtualizationEdgeConfig)
	if !ok {
		return nil, fmt.Errorf("expected a NetworkVirtualizationEdgeConfig object but got %T", obj)
	}
	return nil, nil
}

const maxTotalVLANs = 512

type rng struct {
	start int16
	end   int16
}

// validateNetworkVirtualizationEdgeConfigSpec performs validation to enforce that the VLAN ranges
// - are strictly non overlapping
// - the number of vlans configured does not exceed 512
// - the IDs must be in the range 1-3967
func validateNetworkVirtualizationEdgeConfigSpec(vc *v1alpha1.NetworkVirtualizationEdgeConfig) error {
	if vc.Spec.InfraVLANs == nil {
		return nil
	}

	var vlanRanges []rng
	for _, item := range vc.Spec.InfraVLANs {
		start, end := item.ID, item.ID
		if item.ID == 0 {
			start = item.RangeMin
			end = item.RangeMax
		}
		if end < start {
			return fmt.Errorf("range end < start in (%d-%d)", start, end)
		}

		vlanRanges = append(vlanRanges, rng{start: start, end: end})
	}

	slices.SortFunc(vlanRanges, func(i, j rng) int { return cmp.Compare(i.start, j.start) })
	currVLANs := (vlanRanges[0].end - vlanRanges[0].start + 1)
	for i := 1; i < len(vlanRanges); i++ {
		prev := vlanRanges[i-1]
		cur := vlanRanges[i]
		if cur.start <= prev.end {
			return fmt.Errorf("overlapping vlan ranges (%d-%d) and (%d-%d)", prev.start, prev.end, cur.start, cur.end)
		}
		currVLANs += (cur.end - cur.start + 1)
		if currVLANs > maxTotalVLANs {
			return fmt.Errorf("total number of vlans exceeds maximum of %d", maxTotalVLANs)
		}
	}
	return nil
}
