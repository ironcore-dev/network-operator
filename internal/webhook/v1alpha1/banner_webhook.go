// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupBannerWebhookWithManager registers the Banner validator.
func SetupBannerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.Banner{}).
		WithValidator(&BannerCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-banner,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=banners,verbs=create;update,versions=v1alpha1,name=banner-v1alpha1.kb.io,admissionReviewVersions=v1

// BannerCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type BannerCustomValidator struct{}

func (v *BannerCustomValidator) ValidateCreate(_ context.Context, obj *api.Banner) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *BannerCustomValidator) ValidateUpdate(ctx context.Context, _ *api.Banner, obj *api.Banner) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *BannerCustomValidator) ValidateDelete(_ context.Context, _ *api.Banner) (admission.Warnings, error) {
	return nil, nil
}
