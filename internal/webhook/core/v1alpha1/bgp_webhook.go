// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var bgplog = logf.Log.WithName("bgp-resource")

// SetupBGPWebhookWithManager registers the webhook for BGP in the manager.
func SetupBGPWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.BGP{}).
		WithValidator(&BGPCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-bgp,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=bgp,verbs=create;update,versions=v1alpha1,name=bgp-v1alpha1.kb.io,admissionReviewVersions=v1

// BGPCustomValidator struct is responsible for validating the BGP resource
// when it is created, updated, or deleted.
type BGPCustomValidator struct{}

var _ webhook.CustomValidator = &BGPCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type BGP.
func (v *BGPCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	bgp, ok := obj.(*v1alpha1.BGP)
	if !ok {
		return nil, fmt.Errorf("expected a BGP object but got %T", obj)
	}

	bgplog.Info("Validation for BGP upon creation", "name", bgp.GetName())

	return nil, validateASNumber(bgp.Spec.ASNumber)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type BGP.
func (v *BGPCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	bgp, ok := newObj.(*v1alpha1.BGP)
	if !ok {
		return nil, fmt.Errorf("expected a BGP object for the newObj but got %T", newObj)
	}

	bgplog.Info("Validation for BGP upon update", "name", bgp.GetName())

	return nil, validateASNumber(bgp.Spec.ASNumber)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type BGP.
func (v *BGPCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateASNumber performs validation on the autonomous system number (ASN).
// It ensures the ASN is within valid ranges for both plain and dotted notation as per [RFC 5396].
// [RFC 5396](https://datatracker.ietf.org/doc/html/rfc5396)
func validateASNumber(asn intstr.IntOrString) error {
	// If the value is an integer, validate plain format only
	if asn.Type == intstr.Int {
		asnValue := asn.IntVal
		if asnValue < 1 {
			return fmt.Errorf("AS number %d must be >=1 (for larger values use string format to support full range 1-4294967295)", asnValue)
		}
		return nil
	}

	// If the value is a string, it can be either plain or dotted notation
	asnStr := asn.StrVal

	// Try to parse as plain format first
	if !strings.Contains(asnStr, ".") {
		asnValue, err := strconv.ParseInt(asnStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid AS number format %q: %w", asnStr, err)
		}
		if asnValue < 1 || asnValue > math.MaxUint32 {
			return fmt.Errorf("AS number %d is out of valid range (1-4294967295)", asnValue)
		}
		return nil
	}

	// Parse as dotted notation (high.low)
	parts := strings.Split(asnStr, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid AS number dotted notation %q: must be in format high.low", asnStr)
	}

	high, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid AS number dotted notation %q: high part is not a valid number: %w", asnStr, err)
	}
	if high < 1 || high > math.MaxUint16 {
		return fmt.Errorf("invalid AS number dotted notation %q: high part %d is out of valid range (1-65535)", asnStr, high)
	}

	low, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid AS number dotted notation %q: low part is not a valid number: %w", asnStr, err)
	}
	if low < 0 || low > math.MaxUint16 {
		return fmt.Errorf("invalid AS number dotted notation %q: low part %d is out of valid range (0-65535)", asnStr, low)
	}

	return nil
}
