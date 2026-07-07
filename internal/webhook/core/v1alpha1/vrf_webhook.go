// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var vrflog = logf.Log.WithName("vrf-resource")

// SetupVRFWebhookWithManager registers the webhook for VRF in the manager.
func SetupVRFWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.VRF{}).
		WithValidator(&VRFCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-vrf,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=vrfs,verbs=create;update,versions=v1alpha1,name=vrf-v1alpha1.kb.io,admissionReviewVersions=v1

// VRFCustomValidator struct is responsible for validating the VRF resource
// when it is created, updated, or deleted.
type VRFCustomValidator struct{}

var _ admission.Validator[*v1alpha1.VRF] = &VRFCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type VRF.
func (v *VRFCustomValidator) ValidateCreate(_ context.Context, vrf *v1alpha1.VRF) (admission.Warnings, error) {
	vrflog.Info("Validation for VRF upon creation", "name", vrf.GetName())

	var warnings admission.Warnings
	if vrf.Spec.VNI > 0 { //nolint:staticcheck // handling deprecated field for backward compatibility
		warnings = append(warnings, "spec.vni is deprecated; use the vni field on the EVPNInstance resource instead")
	}

	return warnings, validateVRFSpec(vrf)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type VRF.
func (v *VRFCustomValidator) ValidateUpdate(_ context.Context, _, vrf *v1alpha1.VRF) (admission.Warnings, error) {
	vrflog.Info("Validation for VRF upon update", "name", vrf.GetName())

	var warnings admission.Warnings
	if vrf.Spec.VNI > 0 { //nolint:staticcheck // handling deprecated field for backward compatibility
		warnings = append(warnings, "spec.vni is deprecated; use the vni field on the EVPNInstance resource instead")
	}

	return warnings, validateVRFSpec(vrf)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type VRF.
func (v *VRFCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.VRF) (admission.Warnings, error) {
	return nil, nil
}

func validateVRFSpec(vrf *v1alpha1.VRF) error {
	var errAgg []error

	rd := strings.TrimSpace(vrf.Spec.RouteDistinguisher)
	if rd != "" {
		if err := validateRouteDistinguisher(rd); err != nil {
			errAgg = append(errAgg, fmt.Errorf("invalid route distinguisher value %q: %w", rd, err))
		}
	}

	for _, rt := range vrf.Spec.RouteTargets {
		if err := validateRouteDistinguisher(rt.Value); err != nil {
			errAgg = append(errAgg, fmt.Errorf("invalid route target value %q: %w", rt.Value, err))
		}
	}
	return errors.Join(errAgg...)
}

// validateRouteDistinguisher validates RFC 4364 RD textual forms:
//
//	Type 0: <ASN(1-65534)>:<Number(0-4294967295)>
//	Type 1: <IPv4>:<Number(0-65535)>
//	Type 2: <ASN(65536-4294967294)>:<Number(0-65535)>
//
// Notice the reserved ASNs: 0, 65535, 4294967295
func validateRouteDistinguisher(rd string) error {
	parts := strings.Split(rd, ":")
	if len(parts) != 2 {
		return errors.New("format must be <admin>:<assigned>")
	}
	admin, assignedStr := parts[0], parts[1]

	assigned, err := strconv.ParseUint(assignedStr, 10, 64)
	if err != nil {
		return errors.New("'Assigned Number' must be an unsigned decimal")
	}
	// type 1 check
	if ip, err := netip.ParseAddr(admin); err == nil && ip.Is4() {
		if assigned > math.MaxUint16 {
			return errors.New("type-1 'Assigned Number' is out of range (0–65535)")
		}
		return nil
	}

	// types 0 and 2 checks
	asn, err := strconv.ParseUint(admin, 10, 64)
	if err != nil {
		return errors.New("type-0/type-2 'Administrator' must be an unsigned decimal")
	}

	// Reserved ASNs
	switch asn {
	case 0, math.MaxUint16, math.MaxUint32:
		return fmt.Errorf("ASN %d is reserved and cannot be used", asn)
	}

	// type 0:  ASN 0–65535 + 32-bit number (0–4294967295) with reserved previously checked
	if asn <= math.MaxUint16 && assigned <= math.MaxUint32 {
		return nil
	}
	// type 2: ASN 65536–4294967295 + 16-bit number (0–65535) with reserved previously checked
	if asn <= math.MaxUint32 && assigned <= math.MaxUint16 {
		return nil
	}

	return errors.New("not a valid type-0, type-1, or type-2 RD")
}
