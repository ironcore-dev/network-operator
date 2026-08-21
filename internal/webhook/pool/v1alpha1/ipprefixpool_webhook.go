// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

// log is for logging of the [IPPrefixPoolCustomValidator].
var ipprefixpoollog = logf.Log.WithName("ipprefixpool-resource")

// SetupIPPrefixPoolWebhookWithManager registers the webhook for IPPrefixPool in the manager.
func SetupIPPrefixPoolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &poolv1alpha1.IPPrefixPool{}).
		WithValidator(&IPPrefixPoolCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-pool-networking-metal-ironcore-dev-v1alpha1-ipprefixpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=pool.networking.metal.ironcore.dev,resources=ipprefixpools,verbs=create;update,versions=v1alpha1,name=vipprefixpool-v1alpha1.kb.io,admissionReviewVersions=v1

// IPPrefixPoolCustomValidator struct is responsible for validating the IPPrefixPool resource
// when it is created, updated, or deleted.
type IPPrefixPoolCustomValidator struct{}

var _ admission.Validator[*poolv1alpha1.IPPrefixPool] = &IPPrefixPoolCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type IPPrefixPool.
func (v *IPPrefixPoolCustomValidator) ValidateCreate(_ context.Context, obj *poolv1alpha1.IPPrefixPool) (admission.Warnings, error) {
	ipprefixpoollog.Info("Validation for IPPrefixPool upon creation", "name", obj.GetName())
	return nil, validateIPPrefixPoolSpec(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type IPPrefixPool.
func (v *IPPrefixPoolCustomValidator) ValidateUpdate(_ context.Context, _, newObj *poolv1alpha1.IPPrefixPool) (admission.Warnings, error) {
	ipprefixpoollog.Info("Validation for IPPrefixPool upon update", "name", newObj.GetName())
	return nil, validateIPPrefixPoolSpec(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type IPPrefixPool.
func (v *IPPrefixPoolCustomValidator) ValidateDelete(_ context.Context, _ *poolv1alpha1.IPPrefixPool) (admission.Warnings, error) {
	return nil, nil
}

// validateIPPrefixPoolSpec checks that all base prefixes use the same address family,
// that prefixLength is larger than each base prefix length, and that no two base
// prefixes overlap in address space.
func validateIPPrefixPoolSpec(pool *poolv1alpha1.IPPrefixPool) error {
	if len(pool.Spec.Prefixes) == 0 {
		return nil
	}
	is4 := pool.Spec.Prefixes[0].Addr().Is4()
	for i := range pool.Spec.Prefixes {
		if pool.Spec.Prefixes[i].Addr().Is4() != is4 {
			return fmt.Errorf("spec.prefixes[%d]: cannot mix IPv4 and IPv6 prefixes", i)
		}
		if int(pool.Spec.AllocationPrefixLength) <= pool.Spec.Prefixes[i].Bits() {
			return fmt.Errorf("spec.prefixLength (%d) must be greater than the prefix length of spec.prefixes[%d] (/%d)", pool.Spec.AllocationPrefixLength, i, pool.Spec.Prefixes[i].Bits())
		}
		for j := i + 1; j < len(pool.Spec.Prefixes); j++ {
			if pool.Spec.Prefixes[i].Overlaps(pool.Spec.Prefixes[j].Prefix) {
				return fmt.Errorf("spec.prefixes[%d] (%s) overlaps with spec.prefixes[%d] (%s)", i, pool.Spec.Prefixes[i].Masked(), j, pool.Spec.Prefixes[j].Masked())
			}
		}
	}
	return nil
}
