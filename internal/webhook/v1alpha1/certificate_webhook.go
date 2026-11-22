// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupCertificateWebhookWithManager registers the Certificate validator.
func SetupCertificateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.Certificate{}).
		WithValidator(&CertificateCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-certificate,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=certificates,verbs=create;update,versions=v1alpha1,name=certificate-v1alpha1.kb.io,admissionReviewVersions=v1

// CertificateCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type CertificateCustomValidator struct{}

func (v *CertificateCustomValidator) ValidateCreate(_ context.Context, obj *api.Certificate) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *CertificateCustomValidator) ValidateUpdate(ctx context.Context, _ *api.Certificate, obj *api.Certificate) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *CertificateCustomValidator) ValidateDelete(_ context.Context, _ *api.Certificate) (admission.Warnings, error) {
	return nil, nil
}
