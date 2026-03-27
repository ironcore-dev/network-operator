// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"errors"
	"net/netip"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var nvelog = logf.Log.WithName("networkvirtualizationedge-resource")

// SetupNetworkVirtualizationEdgeWebhookWithManager registers the webhook for NetworkVirtualizationEdge in the manager.
func SetupNetworkVirtualizationEdgeWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.NetworkVirtualizationEdge{}).
		WithValidator(&NetworkVirtualizationEdgeCustomValidator{mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-networkvirtualizationedge,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=networkvirtualizationedges,verbs=create;update,versions=v1alpha1,name=networkvirtualizationedge-v1alpha1.kb.io,admissionReviewVersions=v1
// NetworkVirtualizationEdgeCustomValidator struct is responsible for validating the NetworkVirtualizationEdge resource
// when it is created, updated, or deleted.
type NetworkVirtualizationEdgeCustomValidator struct {
	Client client.Client
}

var _ admission.Validator[*v1alpha1.NetworkVirtualizationEdge] = &NetworkVirtualizationEdgeCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type NetworkVirtualizationEdge.
func (v *NetworkVirtualizationEdgeCustomValidator) ValidateCreate(_ context.Context, nve *v1alpha1.NetworkVirtualizationEdge) (admission.Warnings, error) {
	nvelog.Info("Validation for NetworkVirtualizationEdge upon creation", "name", nve.GetName())

	return nil, v.validateNetworkVirtualizationEdgeSpec(nve)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type NetworkVirtualizationEdge.
func (v *NetworkVirtualizationEdgeCustomValidator) ValidateUpdate(_ context.Context, _, nve *v1alpha1.NetworkVirtualizationEdge) (admission.Warnings, error) {
	nvelog.Info("Validation for NetworkVirtualizationEdge upon update", "name", nve.GetName())

	return nil, v.validateNetworkVirtualizationEdgeSpec(nve)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type NetworkVirtualizationEdge.
func (v *NetworkVirtualizationEdgeCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.NetworkVirtualizationEdge) (admission.Warnings, error) {
	return nil, nil
}

// validateNetworkVirtualizationEdgeSpec performs validation of the NetworkVirtualizationEdge spec, namely on the MulticastGroups field.
func (v *NetworkVirtualizationEdgeCustomValidator) validateNetworkVirtualizationEdgeSpec(nve *v1alpha1.NetworkVirtualizationEdge) error {
	if nve.Spec.MulticastGroups == nil {
		return nil
	}
	if nve.Spec.MulticastGroups.L2 != nil {
		if !v.validateMulticastAddr(nve.Spec.MulticastGroups.L2.Prefix) {
			return errors.New("invalid L2 multicast group: must be a valid IPv4 multicast CIDR with no host bits set")
		}
	}
	if nve.Spec.MulticastGroups.L3 != nil {
		if !v.validateMulticastAddr(nve.Spec.MulticastGroups.L3.Prefix) {
			return errors.New("invalid L3 multicast group: must be a valid IPv4 multicast CIDR with no host bits set")
		}
	}
	return nil
}

// validateMulticastAddr checks if the provided prefix is a valid multicast address with no host bits set.
func (*NetworkVirtualizationEdgeCustomValidator) validateMulticastAddr(pfx netip.Prefix) bool {
	// Check it's a multicast address
	if !pfx.Addr().Is4() || !pfx.Addr().IsMulticast() {
		return false
	}

	// Check no host bits are set (canonical form)
	if pfx.Masked().Addr() != pfx.Addr() {
		return false
	}

	return true
}
