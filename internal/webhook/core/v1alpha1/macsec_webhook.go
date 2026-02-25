// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// macseclog is for logging in this package.
var macseclog = logf.Log.WithName("macsec-resource")

// SetupMacSecWebhookWithManager registers the webhook for MacSec in the manager.
func SetupMacSecWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.MacSec{}).
		WithValidator(&MacSecCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-macsec,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=macsec,verbs=create;update,versions=v1alpha1,name=macsec-v1alpha1.kb.io,admissionReviewVersions=v1

// MacSecCustomValidator struct is responsible for validating the MacSec resource
// when it is created, updated, or deleted.
type MacSecCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type MacSec.
func (v *MacSecCustomValidator) ValidateCreate(_ context.Context, obj *v1alpha1.MacSec) (admission.Warnings, error) {
	macseclog.Info("Validation for MacSec upon creation", "name", obj.GetName())

	for _, psk := range obj.Spec.PreSharedKeys {
		if err := validatePreSharedKeyTimes(&psk); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type MacSec.
func (v *MacSecCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *v1alpha1.MacSec) (admission.Warnings, error) {
	macseclog.Info("Validation for MacSec upon update", "name", newObj.GetName())

	for _, psk := range newObj.Spec.PreSharedKeys {
		if err := validatePreSharedKeyTimes(&psk); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type MacSec.
func (v *MacSecCustomValidator) ValidateDelete(_ context.Context, obj *v1alpha1.MacSec) (admission.Warnings, error) {
	macseclog.Info("Validation for MacSec upon deletion", "name", obj.GetName())
	return nil, nil
}

func validatePreSharedKeyTimes(key *v1alpha1.PreSharedKey) error {
	if key.StartTime.After(key.EndTime.Time) {
		return fmt.Errorf("pre-shared key start time %v is after end time %v", key.StartTime.Time, key.EndTime.Time)
	}

	currTime := time.Now()
	if !key.EndTime.After(currTime) {
		return fmt.Errorf("pre-shared key end time %v is not after current time %v", key.EndTime.Time, currTime)
	}
	return nil
}
