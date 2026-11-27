// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupConsoleConnectionWebhookWithManager registers the ConsoleConnection validator.
func SetupConsoleConnectionWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.ConsoleConnection{}).
		WithValidator(&ConsoleConnectionCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-consoleconnection,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=consoleconnections,verbs=create;update,versions=v1alpha1,name=consoleconnection-v1alpha1.kb.io,admissionReviewVersions=v1

// ConsoleConnectionCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type ConsoleConnectionCustomValidator struct{}

func (v *ConsoleConnectionCustomValidator) ValidateCreate(_ context.Context, obj *api.ConsoleConnection) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *ConsoleConnectionCustomValidator) ValidateUpdate(ctx context.Context, _ *api.ConsoleConnection, obj *api.ConsoleConnection) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *ConsoleConnectionCustomValidator) ValidateDelete(_ context.Context, _ *api.ConsoleConnection) (admission.Warnings, error) {
	return nil, nil
}
