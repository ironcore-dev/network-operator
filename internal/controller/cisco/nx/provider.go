// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"context"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
)

// Provider is the interface that a provider must implement to manage nx specific resources.
type Provider interface {
	provider.Provider

	EnsureSystemSettings(ctx context.Context, s *nxv1alpha1.System) error
	ResetSystemSettings(ctx context.Context) error

	EnsureVPCDomain(ctx context.Context, vpc *nxv1alpha1.VPCDomain, vrf *v1alpha1.VRF, intf *v1alpha1.Interface) error
	DeleteVPCDomain(context.Context) error
	GetStatusVPCDomain(context.Context) (nxos.VPCDomainStatus, error)

	EnsureBorderGatewaySettings(ctx context.Context, req *nxos.BorderGatewaySettingsRequest) error
	ResetBorderGatewaySettings(ctx context.Context) error
}

var _ Provider = (*nxos.Provider)(nil)
