// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"context"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

type Provider struct{}

func (p *Provider) CreateInterface(context.Context, *v1alpha1.Interface) error {
	return provider.ErrUnimplemented
}

func (p *Provider) DeleteInterface(context.Context, *v1alpha1.Interface) error {
	return provider.ErrUnimplemented
}

func (p *Provider) CreateDevice(context.Context, *v1alpha1.Device) error {
	return provider.ErrUnimplemented
}

func (p *Provider) DeleteDevice(context.Context, *v1alpha1.Device) error {
	return provider.ErrUnimplemented
}

func init() {
	provider.Register("cisco-nxos-gnmi", &Provider{})
}
