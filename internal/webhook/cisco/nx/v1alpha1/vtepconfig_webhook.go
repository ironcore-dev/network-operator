// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
)

// vclog is for logging in this package.
var vclog = logf.Log.WithName("vtepconfig-resource")

// SetupVTEPWebhookWithManager registers the webhook for VTEP in the manager.
func SetupVTEPConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.VTEPConfig{}).
		WithValidator(&VTEPConfigCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-nx-cisco-networking-metal-ironcore-dev-v1alpha1-vtepconfig,mutating=false,failurePolicy=Fail,sideEffects=None,groups=nx.cisco.networking.metal.ironcore.dev,resources=vtepconfigs,verbs=create;update,versions=v1alpha1,name=vtepconfig-cisco-nx-v1alpha1.kb.io,admissionReviewVersions=v1

// VTEPConfigCustomValidator struct is responsible for validating the VTEPConfig resource
// when it is created, updated, or deleted.
type VTEPConfigCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &VTEPConfigCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VTEPConfig.
func (v *VTEPConfigCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	vc, ok := obj.(*v1alpha1.VTEPConfig)
	if !ok {
		return nil, fmt.Errorf("expected a VTEPConfig object but got %T", obj)
	}
	vclog.Info("Validation for VTEPConfig upon creation", "name", vc.GetName())

	return nil, validateVTEPConfigSpec(vc)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VTEPConfig.
func (v *VTEPConfigCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	vc, ok := newObj.(*v1alpha1.VTEPConfig)

	if !ok {
		return nil, fmt.Errorf("expected a VTEPConfig object for the newObj but got %T", newObj)
	}
	vclog.Info("Validation for VTEPConfig upon update", "name", vc.GetName())

	return nil, validateVTEPConfigSpec(vc)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VTEPConfig.
func (v *VTEPConfigCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*v1alpha1.VTEPConfig)
	if !ok {
		return nil, fmt.Errorf("expected a VTEPConfig object but got %T", obj)
	}
	return nil, nil
}

const minVLAN = 1
const maxVLAN = 3967
const maxTotalVLANs = 512

type rng struct {
	start uint
	end   uint
}

// validateVTEPConfigSpec performs validation to enforce that the VLAN ranges
// - are strictly non overlapping
// - the number of vlans configured does not exceed 512
// - the IDs must be in the range 1-3967
func validateVTEPConfigSpec(vc *v1alpha1.VTEPConfig) error {
	var vlanRanges []rng

	validateRange := func(start, end uint) error {
		if start < minVLAN || start > maxVLAN || end < minVLAN || end > maxVLAN {
			return fmt.Errorf("vlan ids out of range (%d-%d) in (%d-%d)", minVLAN, maxVLAN, start, end)
		}
		if end < start {
			return fmt.Errorf("range end < start in (%d-%d)", start, end)
		}
		return nil
	}

	for _, item := range vc.Spec.InfraVLANs {
		if item.ID > 0 && (item.RangeMin > 0 || item.RangeMax > 0) {
			return fmt.Errorf("either ID or both rangeMin and rangeMax must be set, found ID %d with rangeMin %d and rangeMax %d", item.ID, item.RangeMin, item.RangeMax)
		}
		start, end := item.ID, item.ID
		if item.ID == 0 {
			start = item.RangeMin
			end = item.RangeMax
		}

		if err := validateRange(start, end); err != nil {
			return err
		}
		vlanRanges = append(vlanRanges, rng{start: start, end: end})
	}

	sort.Slice(vlanRanges, func(i, j int) bool { return vlanRanges[i].start < vlanRanges[j].start })
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
