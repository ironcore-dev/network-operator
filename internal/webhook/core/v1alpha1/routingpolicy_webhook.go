// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var routingpolicylog = logf.Log.WithName("routingpolicy-resource")

// SetupRoutingPolicyWebhookWithManager registers the webhook for RoutingPolicy in the manager.
func SetupRoutingPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.RoutingPolicy{}).
		WithValidator(&RoutingPolicyCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-routingpolicy,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=routingpolicies,verbs=create;update,versions=v1alpha1,name=routingpolicy-v1alpha1.kb.io,admissionReviewVersions=v1

// RoutingPolicyCustomValidator struct is responsible for validating the RoutingPolicy resource
// when it is created, updated, or deleted.
type RoutingPolicyCustomValidator struct{}

var _ admission.Validator[*v1alpha1.RoutingPolicy] = &RoutingPolicyCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type RoutingPolicy.
func (v *RoutingPolicyCustomValidator) ValidateCreate(_ context.Context, obj *v1alpha1.RoutingPolicy) (admission.Warnings, error) {
	routingpolicylog.Info("Validation for RoutingPolicy upon creation", "name", obj.GetName())

	return nil, validateRoutingPolicyASNumbers(obj)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type RoutingPolicy.
func (v *RoutingPolicyCustomValidator) ValidateUpdate(_ context.Context, _, obj *v1alpha1.RoutingPolicy) (admission.Warnings, error) {
	routingpolicylog.Info("Validation for RoutingPolicy upon update", "name", obj.GetName())

	return nil, validateRoutingPolicyASNumbers(obj)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type RoutingPolicy.
func (v *RoutingPolicyCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.RoutingPolicy) (admission.Warnings, error) {
	return nil, nil
}

// validateRoutingPolicyASNumbers validates all AS numbers referenced in the RoutingPolicy statements.
func validateRoutingPolicyASNumbers(policy *v1alpha1.RoutingPolicy) error {
	for i, stmt := range policy.Spec.Statements {
		if stmt.Actions.BgpActions == nil || stmt.Actions.BgpActions.SetASPath == nil {
			continue
		}

		asPath := stmt.Actions.BgpActions.SetASPath

		if asPath.ASNumber != nil {
			if err := validateASNumber(*asPath.ASNumber); err != nil {
				return fmt.Errorf("statement[%d].actions.bgpActions.setASPath.asNumber: %w", i, err)
			}
		}

		if asPath.Prepend != nil && asPath.Prepend.ASNumber != nil {
			if err := validateASNumber(*asPath.Prepend.ASNumber); err != nil {
				return fmt.Errorf("statement[%d].actions.bgpActions.setASPath.prepend.asNumber: %w", i, err)
			}
		}

		if asPath.Replace != nil {
			if asPath.Replace.ASNumber != nil {
				if err := validateASNumber(*asPath.Replace.ASNumber); err != nil {
					return fmt.Errorf("statement[%d].actions.bgpActions.setASPath.replace.asNumber: %w", i, err)
				}
			}

			if err := validateASNumber(asPath.Replace.Replacement); err != nil {
				return fmt.Errorf("statement[%d].actions.bgpActions.setASPath.replace.replacement: %w", i, err)
			}
		}
	}

	return nil
}
