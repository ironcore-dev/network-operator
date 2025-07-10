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
	panic("unimplemented")
}

func (p *Provider) DeleteInterface(context.Context, *v1alpha1.Interface) error {
	panic("unimplemented")
}

func init() {
	provider.Register("cisco-nxos-gnmi", &Provider{})
}
