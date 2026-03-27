// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var bgppeerlog = logf.Log.WithName("bgppeer-resource")

// SetupBGPPeerWebhookWithManager registers the webhook for BGPPeer in the manager.
func SetupBGPPeerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.BGPPeer{}).
		WithValidator(&BGPPeerCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-bgppeer,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=bgppeers,verbs=create;update,versions=v1alpha1,name=bgppeer-v1alpha1.kb.io,admissionReviewVersions=v1

// BGPPeerCustomValidator struct is responsible for validating the BGPPeer resource
// when it is created, updated, or deleted.
type BGPPeerCustomValidator struct{}

var _ admission.Validator[*v1alpha1.BGPPeer] = &BGPPeerCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type BGPPeer.
func (v *BGPPeerCustomValidator) ValidateCreate(_ context.Context, bgppeer *v1alpha1.BGPPeer) (admission.Warnings, error) {
	bgppeerlog.Info("Validation for BGPPeer upon creation", "name", bgppeer.GetName())

	return nil, validateASNumber(bgppeer.Spec.ASNumber)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type BGPPeer.
func (v *BGPPeerCustomValidator) ValidateUpdate(_ context.Context, _, bgppeer *v1alpha1.BGPPeer) (admission.Warnings, error) {
	bgppeerlog.Info("Validation for BGPPeer upon update", "name", bgppeer.GetName())

	return nil, validateASNumber(bgppeer.Spec.ASNumber)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type BGPPeer.
func (v *BGPPeerCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.BGPPeer) (admission.Warnings, error) {
	return nil, nil
}
