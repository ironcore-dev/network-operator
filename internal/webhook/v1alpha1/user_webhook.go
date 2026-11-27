// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupUserWebhookWithManager registers the User validator.
func SetupUserWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.User{}).
		WithValidator(&UserCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-user,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=users,verbs=create;update,versions=v1alpha1,name=user-v1alpha1.kb.io,admissionReviewVersions=v1

// UserCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type UserCustomValidator struct{}

func (v *UserCustomValidator) ValidateCreate(_ context.Context, obj *api.User) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *UserCustomValidator) ValidateUpdate(ctx context.Context, _ *api.User, obj *api.User) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *UserCustomValidator) ValidateDelete(_ context.Context, _ *api.User) (admission.Warnings, error) {
	return nil, nil
}
