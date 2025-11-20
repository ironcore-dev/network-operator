// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var vteplog = logf.Log.WithName("vtep-resource")

// SetupVTEPWebhookWithManager registers the webhook for VTEP in the manager.
func SetupVTEPWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.VTEP{}).
		WithValidator(&VTEPCustomValidator{mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-vtep,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=vteps,verbs=create;update,versions=v1alpha1,name=vtep-v1alpha1.kb.io,admissionReviewVersions=v1

// VTEPCustomValidator struct is responsible for validating the VTEP resource
// when it is created, updated, or deleted. It validates:
//   - If multicastGroup is specified, its prefix must be a valid multicast address (currently we only check that the first and last address in the prefix are multicast addresses).
type VTEPCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &VTEPCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VTEP.
func (v *VTEPCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	vtep, ok := obj.(*v1alpha1.VTEP)
	if !ok {
		return nil, fmt.Errorf("expected a VTEP object but got %T", obj)
	}
	vteplog.Info("Validation for VTEP upon creation", "name", vtep.GetName())

	return nil, v.validateVTEPSpec(vtep)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VTEP.
func (v *VTEPCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	vtep, ok := newObj.(*v1alpha1.VTEP)
	if !ok {
		return nil, fmt.Errorf("expected a VTEP object for the newObj but got %T", newObj)
	}
	vteplog.Info("Validation for VTEP upon update", "name", vtep.GetName())

	return nil, v.validateVTEPSpec(vtep)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VTEP.
func (v *VTEPCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*v1alpha1.VTEP)
	if !ok {
		return nil, fmt.Errorf("expected a VTEP object but got %T", obj)
	}

	return nil, nil
}

// validateVTEPSpec performs validation of the VTEP spec. The validation rules include:
// - If multicastGroup is specified, its prefix must be a valid multicast address (we only check that the first and last address in the prefix are multicast addresses).
func (v *VTEPCustomValidator) validateVTEPSpec(vtep *v1alpha1.VTEP) error {
	if vtep.Spec.MulticastGroup != nil {
		if !vtep.Spec.MulticastGroup.Prefix.First().IsMulticast() {
			return fmt.Errorf("multicastGroup prefix first address %q is not a valid multicast address", vtep.Spec.MulticastGroup.Prefix.String())
		}
		if !vtep.Spec.MulticastGroup.Prefix.Last().IsMulticast() {
			return fmt.Errorf("multicastGroup prefix last address %q is not a valid multicast address", vtep.Spec.MulticastGroup.Prefix.String())
		}
	}
	return nil
}
