// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"

	"github.com/ironcore-dev/network-operator/internal/provider/api"
)

// DeviceProvider defines a standard interface for managing a network device
// and all of its sub-components, like interfaces. It abstracts away
// vendor-specific implementation details.
type DeviceProvider interface {
	// --- Connection & Discovery ---
	Connect(ctx context.Context, details api.ConnectionDetails) error
	Disconnect(ctx context.Context) error
	GetDeviceInfo(ctx context.Context) (api.DeviceInfo, error)
	ListPhysicalInterfaces(ctx context.Context) ([]api.Interface, error)

	// --- Device-Wide Configuration ---
	EnsureDeviceSettings(ctx context.Context, config api.DeviceSettingsConfig) error

	// fine grained configuration
	EnsureVLAN(ctx context.Context, config api.VLANConfig) error
	DeleteVLAN(ctx context.Context, vlanID int) error
	EnsureLAG(ctx context.Context, config api.LAGConfig) error
	DeleteLAG(ctx context.Context, name string) error
	EnsureBGPConfig(ctx context.Context, config api.BGPConfig) error
	EnsureBGPNeighbor(ctx context.Context, config api.BGPNeighborConfig) error
	DeleteBGPNeighbor(ctx context.Context, neighborAddress string) error

	// coarse-grained configuration
	EnsureDeviceConfig(ctx context.Context, config api.DeviceConfig) error

	// --- Interface-Specific Configuration ---
	EnsureInterfaceConfig(ctx context.Context, config api.InterfaceConfig) error
	GetInterface(ctx context.Context, name string) (api.Interface, error)
	EnsureLoopbackConfig(ctx context.Context, config api.LoopbackInterfaceConfig) error
	GetLoopbackInterface(ctx context.Context, name string) (api.LoopbackInterfaceConfig, error)
	DeleteLoopback(ctx context.Context, name string) error
	EnableInterface(ctx context.Context, name string) error
	DisableInterface(ctx context.Context, name string) error
}
