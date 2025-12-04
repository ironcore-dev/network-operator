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
var prefixsetlog = logf.Log.WithName("prefixset-resource")

// SetupPrefixSetWebhookWithManager registers the webhook for PrefixSets in the manager.
func SetupPrefixSetWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.PrefixSet{}).
		WithValidator(&PrefixSetCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-prefixset,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=prefixsets,verbs=create;update,versions=v1alpha1,name=prefixset-v1alpha1.kb.io,admissionReviewVersions=v1

// PrefixSetCustomValidator struct is responsible for validating the PrefixSet resource
// when it is created, updated, or deleted.
type PrefixSetCustomValidator struct{}

var _ webhook.CustomValidator = &PrefixSetCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PrefixSet.
func (v *PrefixSetCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	ps, ok := obj.(*v1alpha1.PrefixSet)
	if !ok {
		return nil, fmt.Errorf("expected a PrefixSets object but got %T", obj)
	}

	prefixsetlog.Info("Validation for PrefixSets upon creation", "name", ps.GetName())

	return nil, validatePrefixSetSpec(ps)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PrefixSet.
func (v *PrefixSetCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	prev, ok := oldObj.(*v1alpha1.PrefixSet)
	if !ok {
		return nil, fmt.Errorf("expected a PrefixSets object for the oldObj but got %T", oldObj)
	}

	curr, ok := newObj.(*v1alpha1.PrefixSet)
	if !ok {
		return nil, fmt.Errorf("expected a PrefixSets object for the newObj but got %T", newObj)
	}

	prefixsetlog.Info("Validation for PrefixSets upon update", "name", curr.GetName())

	if err := validatePrefixSetSpec(curr); err != nil {
		return nil, err
	}

	if len(prev.Spec.Entries) > 0 && prev.Is6() != curr.Is6() {
		return nil, errors.New("cannot change IP family of a PrefixSet once created")
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PrefixSet.
func (v *PrefixSetCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validatePrefixSetSpec performs validation on the PrefixSet spec.
func validatePrefixSetSpec(ps *v1alpha1.PrefixSet) error {
	var is6 bool
	var errAgg []error
	for i, ent := range ps.Spec.Entries {
		if i > 0 && ent.Prefix.Addr().Is6() != is6 {
			errAgg = append(errAgg, errors.New("all prefixes in a PrefixSet must be of the same IP family"))
		}
		is6 = ent.Prefix.Addr().Is6()
		if ent.MaskLengthRange != nil {
			bits := ent.Prefix.Bits()
			rmin := int(ent.MaskLengthRange.Min)
			rmax := int(ent.MaskLengthRange.Max)

			if rmin < bits {
				errAgg = append(errAgg, fmt.Errorf("entry %d: mask length range min %d is invalid for prefix %s", i, rmin, ent.Prefix.String()))
			}
			if rmax < bits {
				errAgg = append(errAgg, fmt.Errorf("entry %d: mask length range max %d is invalid for prefix %s", i, rmax, ent.Prefix.String()))
			}
			if rmin > rmax {
				errAgg = append(errAgg, fmt.Errorf("entry %d: mask length range min %d cannot be greater than max %d", i, rmin, rmax))
			}
			const maxBits = 32
			if !is6 && rmin > maxBits {
				errAgg = append(errAgg, fmt.Errorf("entry %d: mask length range min %d exceeds maximum %d bits for IPv4", i, rmin, maxBits))
			}
			if !is6 && rmax > maxBits {
				errAgg = append(errAgg, fmt.Errorf("entry %d: mask length range max %d exceeds maximum %d bits for IPv4", i, rmax, maxBits))
			}
		}
	}
	return errors.Join(errAgg...)
}
