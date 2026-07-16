// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"math"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

// log is for logging of the [IndexPoolCustomValidator].
var indexpoollog = logf.Log.WithName("indexpool-resource")

// SetupIndexPoolWebhookWithManager registers the webhook for IndexPool in the manager.
func SetupIndexPoolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &poolv1alpha1.IndexPool{}).
		WithValidator(&IndexPoolCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-pool-networking-metal-ironcore-dev-v1alpha1-indexpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=pool.networking.metal.ironcore.dev,resources=indexpools,verbs=create;update,versions=v1alpha1,name=vindexpool-v1alpha1.kb.io,admissionReviewVersions=v1

// IndexPoolCustomValidator struct is responsible for validating the IndexPool resource
// when it is created, updated, or deleted.
type IndexPoolCustomValidator struct{}

var _ admission.Validator[*poolv1alpha1.IndexPool] = &IndexPoolCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type IndexPool.
func (v *IndexPoolCustomValidator) ValidateCreate(_ context.Context, obj *poolv1alpha1.IndexPool) (admission.Warnings, error) {
	indexpoollog.Info("Validation for IndexPool upon creation", "name", obj.GetName())
	return nil, validateIndexPoolSpec(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type IndexPool.
func (v *IndexPoolCustomValidator) ValidateUpdate(_ context.Context, _, newObj *poolv1alpha1.IndexPool) (admission.Warnings, error) {
	indexpoollog.Info("Validation for IndexPool upon update", "name", newObj.GetName())
	return nil, validateIndexPoolSpec(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type IndexPool.
func (v *IndexPoolCustomValidator) ValidateDelete(_ context.Context, _ *poolv1alpha1.IndexPool) (admission.Warnings, error) {
	return nil, nil
}

// validateIndexPoolSpec checks that no two ranges overlap and that no range end is math.MaxInt64.
func validateIndexPoolSpec(pool *poolv1alpha1.IndexPool) error {
	for i, a := range pool.Spec.Ranges {
		if a.End == math.MaxInt64 {
			return fmt.Errorf("spec.ranges[%d]: end must be less than math.MaxInt64", i)
		}
		for j := i + 1; j < len(pool.Spec.Ranges); j++ {
			b := pool.Spec.Ranges[j]
			if a.Start <= b.End && b.Start <= a.End {
				return fmt.Errorf("spec.ranges[%d] (%d..%d) overlaps with spec.ranges[%d] (%d..%d)", i, a.Start, a.End, j, b.Start, b.End)
			}
		}
	}
	return nil
}
