// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupAAAWebhookWithManager registers the AAA validator.
func SetupAAAWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.AAA{}).
		WithValidator(&AAACustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-aaa,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=aaa,verbs=create;update,versions=v1alpha1,name=aaa-v1alpha1.kb.io,admissionReviewVersions=v1

// AAACustomValidator restricts Secret and ConfigMap references to the object's namespace.
type AAACustomValidator struct{}

func (v *AAACustomValidator) ValidateCreate(_ context.Context, obj *api.AAA) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *AAACustomValidator) ValidateUpdate(ctx context.Context, _ *api.AAA, obj *api.AAA) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *AAACustomValidator) ValidateDelete(_ context.Context, _ *api.AAA) (admission.Warnings, error) {
	return nil, nil
}
