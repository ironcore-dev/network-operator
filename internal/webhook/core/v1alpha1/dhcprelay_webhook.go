// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var dhcpRelaylog = logf.Log.WithName("dhcprelay-resource")

// SetupDHCPRelayWebhookWithManager registers the webhook for DHCPRelay in the manager.
func SetupDHCPRelayWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.DHCPRelay{}).
		WithValidator(&DHCPRelayCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-dhcprelay,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=dhcprelays,verbs=create;update,versions=v1alpha1,name=dhcprelay-v1alpha1.kb.io,admissionReviewVersions=v1

// DHCPRelayCustomValidator struct is responsible for validating the DHCPRelay resource
// when it is created, updated, or deleted.
type DHCPRelayCustomValidator struct{}

var _ admission.Validator[*v1alpha1.DHCPRelay] = &DHCPRelayCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type DHCPRelay.
func (v *DHCPRelayCustomValidator) ValidateCreate(_ context.Context, DHCPRelay *v1alpha1.DHCPRelay) (admission.Warnings, error) {
	dhcpRelaylog.Info("Validation for DHCPRelay upon creation", "name", DHCPRelay.GetName())

	var warnings admission.Warnings
	if len(DHCPRelay.Spec.InterfaceRefs) > 0 { //nolint:staticcheck // handling deprecated field for backward compatibility
		warnings = append(warnings, "spec.interfaceRefs is deprecated; use the interfaceRef field on the DHCPRelay resource instead")
	}

	return warnings, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type DHCPRelay.
func (v *DHCPRelayCustomValidator) ValidateUpdate(_ context.Context, _, DHCPRelay *v1alpha1.DHCPRelay) (admission.Warnings, error) {
	dhcpRelaylog.Info("Validation for DHCPRelay upon update", "name", DHCPRelay.GetName())

	var warnings admission.Warnings
	if len(DHCPRelay.Spec.InterfaceRefs) > 0 { //nolint:staticcheck // handling deprecated field for backward compatibility
		warnings = append(warnings, "spec.interfaceRefs is deprecated; use the interfaceRef field on the DHCPRelay resource instead")
	}

	return warnings, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type DHCPRelay.
func (v *DHCPRelayCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.DHCPRelay) (admission.Warnings, error) {
	return nil, nil
}
