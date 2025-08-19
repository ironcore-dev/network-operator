// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package edgecore

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/api"
)

// Provider implements the DeviceProvider interface for Edgecore network devices.
// Its internal fields are unexported to encapsulate its implementation details.
type Provider struct {
	log    logr.Logger
	client *http.Client
	// sessionToken would store an auth token after connecting.
	sessionToken string
}

// Config holds the necessary configuration for creating a new EdgeCoreProvider.
type Config struct {
	// Timeout specifies the timeout for API requests to the Edgecore device.
	Timeout time.Duration
}

// NewProvider is a constructor function that creates and initializes
// a provider for interacting with Edgecore devices.
func NewProvider(log logr.Logger, config Config) (provider.DeviceProvider, error) {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return &Provider{
		log: log,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

func (p *Provider) Connect(ctx context.Context, details api.ConnectionDetails) error {
	p.log.Info("Connecting to Edgecore device", "Address", details.Address)
	// token, err := p.authenticate(ctx, address, username, password)
	// if err != nil {
	//     return err
	// }
	// p.sessionToken = token
	p.sessionToken = "fake-edgecore-api-token" // Placeholder
	return nil
}

func (p *Provider) Disconnect(ctx context.Context) error {
	p.log.Info("Disconnecting from Edgecore device...")
	p.sessionToken = ""
	// disconnect logic would go here, such as invalidating the session token.
	return nil
}

func (p *Provider) GetDeviceInfo(ctx context.Context) (api.DeviceInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) ListPhysicalInterfaces(ctx context.Context) ([]api.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureDeviceSettings(ctx context.Context, config api.DeviceSettingsConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureInterfaceConfig(ctx context.Context, config api.InterfaceConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureLoopbackConfig(ctx context.Context, config api.LoopbackInterfaceConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DeleteLoopback(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureVLAN(ctx context.Context, config api.VLANConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DeleteVLAN(ctx context.Context, vlanID int) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureLAG(ctx context.Context, config api.LAGConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DeleteLAG(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureBGPConfig(ctx context.Context, config api.BGPConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureBGPNeighbor(ctx context.Context, config api.BGPNeighborConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DeleteBGPNeighbor(ctx context.Context, neighborAddress string) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) GetInterface(ctx context.Context, name string) (api.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnsureDeviceConfig(ctx context.Context, config api.DeviceConfig) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) GetLoopbackInterface(ctx context.Context, name string) (api.LoopbackInterfaceConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) EnableInterface(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DisableInterface(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}
