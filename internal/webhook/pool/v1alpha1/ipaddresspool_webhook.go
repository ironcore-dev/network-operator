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

// log is for logging of the [IPAddressPoolCustomValidator].
var ipaddresspoollog = logf.Log.WithName("ipaddresspool-resource")

// SetupIPAddressPoolWebhookWithManager registers the webhook for IPAddressPool in the manager.
func SetupIPAddressPoolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &poolv1alpha1.IPAddressPool{}).
		WithValidator(&IPAddressPoolCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-pool-networking-metal-ironcore-dev-v1alpha1-ipaddresspool,mutating=false,failurePolicy=fail,sideEffects=None,groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=create;update,versions=v1alpha1,name=vipaddresspool-v1alpha1.kb.io,admissionReviewVersions=v1

// IPAddressPoolCustomValidator struct is responsible for validating the IPAddressPool resource
// when it is created, updated, or deleted.
type IPAddressPoolCustomValidator struct{}

var _ admission.Validator[*poolv1alpha1.IPAddressPool] = &IPAddressPoolCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type IPAddressPool.
func (v *IPAddressPoolCustomValidator) ValidateCreate(_ context.Context, obj *poolv1alpha1.IPAddressPool) (admission.Warnings, error) {
	ipaddresspoollog.Info("Validation for IPAddressPool upon creation", "name", obj.GetName())
	return nil, validateIPAddressPoolSpec(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type IPAddressPool.
func (v *IPAddressPoolCustomValidator) ValidateUpdate(_ context.Context, _, newObj *poolv1alpha1.IPAddressPool) (admission.Warnings, error) {
	ipaddresspoollog.Info("Validation for IPAddressPool upon update", "name", newObj.GetName())
	return nil, validateIPAddressPoolSpec(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type IPAddressPool.
func (v *IPAddressPoolCustomValidator) ValidateDelete(_ context.Context, _ *poolv1alpha1.IPAddressPool) (admission.Warnings, error) {
	return nil, nil
}

// validateIPAddressPoolSpec checks that all prefixes use the same address family
// and that no two prefixes overlap in address space.
func validateIPAddressPoolSpec(pool *poolv1alpha1.IPAddressPool) error {
	if len(pool.Spec.Prefixes) == 0 {
		return nil
	}
	is4 := pool.Spec.Prefixes[0].Addr().Is4()
	for i := range pool.Spec.Prefixes {
		if pool.Spec.Prefixes[i].Addr().Is4() != is4 {
			return fmt.Errorf("spec.prefixes[%d]: cannot mix IPv4 and IPv6 prefixes", i)
		}
		for j := i + 1; j < len(pool.Spec.Prefixes); j++ {
			if pool.Spec.Prefixes[i].Overlaps(pool.Spec.Prefixes[j].Prefix) {
				return fmt.Errorf("spec.prefixes[%d] (%s) overlaps with spec.prefixes[%d] (%s)", i, pool.Spec.Prefixes[i].Masked(), j, pool.Spec.Prefixes[j].Masked())
			}
		}
	}
	return nil
}
