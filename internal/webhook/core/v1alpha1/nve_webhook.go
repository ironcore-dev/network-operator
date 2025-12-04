// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"net/netip"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var nvelog = logf.Log.WithName("nve-resource")

// SetupNVEWebhookWithManager registers the webhook for NVE in the manager.
func SetupNVEWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.NVE{}).
		WithValidator(&NVECustomValidator{mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-nve,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=nves,verbs=create;update,versions=v1alpha1,name=nve-v1alpha1.kb.io,admissionReviewVersions=v1

// NVECustomValidator struct is responsible for validating the NVE resource
// when it is created, updated, or deleted.
type NVECustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &NVECustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type NVE.
func (v *NVECustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	nve, ok := obj.(*v1alpha1.NVE)
	if !ok {
		return nil, fmt.Errorf("expected a NVE object but got %T", obj)
	}
	nvelog.Info("Validation for NVE upon creation", "name", nve.GetName())

	return nil, v.validateNVESpec(nve)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type NVE.
func (v *NVECustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	nve, ok := newObj.(*v1alpha1.NVE)
	if !ok {
		return nil, fmt.Errorf("expected a NVE object for the newObj but got %T", newObj)
	}
	nvelog.Info("Validation for NVE upon update", "name", nve.GetName())

	return nil, v.validateNVESpec(nve)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type NVE.
func (v *NVECustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*v1alpha1.NVE)
	if !ok {
		return nil, fmt.Errorf("expected a NVE object but got %T", obj)
	}

	return nil, nil
}

// validateNVESpec performs validation of the NVE spec, namely on the MulticastGroups field.
func (v *NVECustomValidator) validateNVESpec(nve *v1alpha1.NVE) error {
	if nve.Spec.MulticastGroups == nil {
		return nil
	}
	if nve.Spec.MulticastGroups.L2 != "" {
		if ok, err := v.isMulticast(nve.Spec.MulticastGroups.L2); err != nil || !ok {
			return fmt.Errorf("%q is not a multicast address", nve.Spec.MulticastGroups.L2)
		}
	}
	if nve.Spec.MulticastGroups.L3 != "" {
		if ok, err := v.isMulticast(nve.Spec.MulticastGroups.L3); err != nil || !ok {
			return fmt.Errorf("%q is not a multicast address", nve.Spec.MulticastGroups.L3)
		}
	}
	return nil
}

func (*NVECustomValidator) isMulticast(s string) (bool, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil || !addr.IsValid() {
		return false, fmt.Errorf("%q is not a valid IP addr: %w", s, err)
	}
	return addr.IsMulticast(), nil
}
