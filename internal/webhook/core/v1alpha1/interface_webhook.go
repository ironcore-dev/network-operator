// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var interfacelog = logf.Log.WithName("interface-resource")

// SetupInterfaceWebhookWithManager registers the webhook for Interfaces in the manager.
func SetupInterfaceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Interface{}).
		WithValidator(&InterfaceCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-interface,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=interfaces,verbs=create;update,versions=v1alpha1,name=interface-v1alpha1.kb.io,admissionReviewVersions=v1

// InterfaceCustomValidator struct is responsible for validating the Interface resource
// when it is created, updated, or deleted.
type InterfaceCustomValidator struct{}

var _ webhook.CustomValidator = &InterfaceCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Interface.
func (v *InterfaceCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		return nil, fmt.Errorf("expected a Interfaces object but got %T", obj)
	}

	interfacelog.Info("Validation for Interfaces upon creation", "name", intf.GetName())

	return nil, validateInterfaceSpec(intf)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Interface.
func (v *InterfaceCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	intf, ok := newObj.(*v1alpha1.Interface)
	if !ok {
		return nil, fmt.Errorf("expected a Interfaces object for the newObj but got %T", newObj)
	}

	interfacelog.Info("Validation for Interfaces upon update", "name", intf.GetName())

	return nil, validateInterfaceSpec(intf)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Interface.
func (v *InterfaceCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateInterfaceSpec performs validation on the Interface spec.
func validateInterfaceSpec(intf *v1alpha1.Interface) error {
	if intf.Spec.IPv4 == nil {
		return nil
	}

	return validateInterfaceIPv4(intf.Spec.IPv4)
}

// validateInterfaceIPv4 performs validation on the InterfaceIPv4 spec.
func validateInterfaceIPv4(ip *v1alpha1.InterfaceIPv4) error {
	var errAgg []error
	for i, cidr := range ip.Addresses {
		if cidr.Prefix.Addr().Is6() {
			errAgg = append(errAgg, fmt.Errorf("invalid IPv4 address %q: address is IPv6", cidr.String()))
			continue
		}
		for j := i + 1; j < len(ip.Addresses); j++ {
			if p := ip.Addresses[j].Prefix; cidr.Overlaps(p) {
				errAgg = append(errAgg, fmt.Errorf("invalid IPv4 address %q: overlaps with %q", cidr.String(), p.String()))
			}
		}
	}
	return errors.Join(errAgg...)
}
