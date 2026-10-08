// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"fmt"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider"
)

var _ provider.DeviceProvider = (*Provider)(nil)

func (p *Provider) GetDeviceInfo(ctx context.Context) (*provider.DeviceInfo, error) {
	name := new(Name)
	// Hostname is a config item, not state.
	if err := p.client.GetConfig(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	chassis := new(ChassisState)
	info := new(SystemInformation)
	if err := p.client.GetState(ctx, chassis, info); err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	return &provider.DeviceInfo{
		Hostname:        name.HostName,
		Manufacturer:    Manufacturer,
		Model:           chassis.Type,
		SerialNumber:    chassis.SerialNumber,
		FirmwareVersion: info.Version,
	}, nil
}

func (p *Provider) GetLastRebootTime(ctx context.Context) (time.Time, error) {
	info := new(SystemInformation)
	if err := p.client.GetState(ctx, info); err != nil {
		return time.Time{}, fmt.Errorf("failed to get last reboot time: %w", err)
	}
	if info.LastBooted == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, info.LastBooted)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse last-booted: %w", err)
	}
	return t, nil
}

func (p *Provider) ListPorts(ctx context.Context) ([]provider.DevicePort, error) {
	ifaces := new(Interfaces)
	if err := p.client.GetOperational(ctx, ifaces); err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	ports := make([]provider.DevicePort, 0, len(ifaces.Interface))
	for _, iface := range ifaces.Interface {
		port := provider.DevicePort{
			ID:   iface.Name,
			Type: iface.Ethernet.PortSpeed,
		}
		ports = append(ports, port)
	}

	return ports, nil
}
