// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupConfigBackupWebhookWithManager registers the ConfigBackup validator.
func SetupConfigBackupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.ConfigBackup{}).
		WithValidator(&ConfigBackupCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-configbackup,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=configbackups,verbs=create;update,versions=v1alpha1,name=configbackup-v1alpha1.kb.io,admissionReviewVersions=v1

// ConfigBackupCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type ConfigBackupCustomValidator struct{}

func (v *ConfigBackupCustomValidator) ValidateCreate(_ context.Context, obj *api.ConfigBackup) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *ConfigBackupCustomValidator) ValidateUpdate(ctx context.Context, _ *api.ConfigBackup, obj *api.ConfigBackup) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *ConfigBackupCustomValidator) ValidateDelete(_ context.Context, _ *api.ConfigBackup) (admission.Warnings, error) {
	return nil, nil
}
