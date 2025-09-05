// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iosxr

import (
	"context"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Provider struct{}

func (p *Provider) CreateDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "CreateDevice is not implemented for iosxr provider")
	return nil
}

func (p *Provider) DeleteDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "DeleteDevice is not implemented for iosxr provider")
	return nil
}

func (p *Provider) CreateInterface(ctx context.Context, intf *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "CreateInterface is not implemented for iosxr provider")
	return nil
}

func (p *Provider) DeleteInterface(ctx context.Context, intf *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "DeleteInterface is not implemented for iosxr provider")
	return nil
}

func init() {
	log := ctrl.Log.WithName("iosxr")
	log.Info("Registering iosxr provider")
	provider.Register("cisco-ios-xr-gnmi", &Provider{})
}
