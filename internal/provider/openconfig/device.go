// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.DeviceProvider = (*Provider)(nil)

func (p *Provider) GetDeviceInfo(ctx context.Context) (*provider.DeviceInfo, error) {
	sys := new(SystemState)
	chassis := new(ChassisComponentState)
	if err := p.client.GetState(ctx, sys, chassis); err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	return &provider.DeviceInfo{
		Hostname:        sys.Hostname,
		Manufacturer:    chassis.MfgName,
		Model:           chassis.ModelName,
		SerialNumber:    chassis.SerialNo,
		FirmwareVersion: sys.SoftwareVersion,
	}, nil
}

func (p *Provider) GetLastRebootTime(ctx context.Context) (time.Time, error) {
	sys := new(SystemState)
	if err := p.client.GetState(ctx, sys); err != nil {
		return time.Time{}, fmt.Errorf("failed to get last reboot time: %w", err)
	}
	if sys.BootTime == "" {
		return time.Time{}, nil
	}
	ns, err := strconv.ParseInt(sys.BootTime, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse boot-time: %w", err)
	}
	return time.Unix(0, ns), nil
}

func (p *Provider) ListPorts(ctx context.Context) ([]provider.DevicePort, error) {
	ports := new(PhysicalInterfaces)
	if err := p.client.GetState(ctx, ports); err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	dp := make([]provider.DevicePort, 0, len(ports.Interface))
	for _, iface := range ports.Interface {
		if iface.State == nil || iface.State.Type != InterfaceTypeEthernetCsmacd {
			continue
		}
		port := provider.DevicePort{
			ID:   iface.Name,
			Type: iface.PortSpeed().Short(),
		}
		dp = append(dp, port)
	}

	return dp, nil
}

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*SystemState)(nil)
	_ gnmiext.DataElement = (*ChassisComponentState)(nil)
	_ gnmiext.DataElement = (*PhysicalInterfaces)(nil)
)

// SystemState maps the openconfig system state container.
type SystemState struct {
	Hostname        string `json:"hostname,omitempty"`
	BootTime        string `json:"boot-time,omitempty"`
	SoftwareVersion string `json:"software-version,omitempty"`
}

func (*SystemState) XPath() string {
	return "openconfig-system:system/state"
}

// ChassisComponentState maps the openconfig platform Chassis component state.
type ChassisComponentState struct {
	MfgName   string `json:"mfg-name,omitempty"`
	ModelName string `json:"model-name,omitempty"`
	SerialNo  string `json:"serial-no,omitempty"`
}

func (*ChassisComponentState) XPath() string {
	return "openconfig-platform:components/component[name=Chassis]/state"
}

// PhysicalInterfaces retrieves the interface list for port discovery.
type PhysicalInterfaces struct {
	Interface []*PhysicalInterfaceEntry `json:"interface,omitempty"`
}

func (*PhysicalInterfaces) XPath() string {
	return "openconfig-interfaces:interfaces"
}

// PhysicalInterfaceEntry represents a single interface for port listing.
type PhysicalInterfaceEntry struct {
	Name     string                     `json:"name"`
	State    *PhysicalInterfaceState    `json:"state,omitempty"`
	Ethernet *PhysicalInterfaceEthernet `json:"openconfig-if-ethernet:ethernet,omitempty"`
}

// PortSpeed returns the ethernet port speed, or [EthernetSpeedUnknown] if unavailable.
func (e *PhysicalInterfaceEntry) PortSpeed() EthernetSpeed {
	if e.Ethernet != nil && e.Ethernet.State != nil {
		return e.Ethernet.State.PortSpeed
	}
	return EthernetSpeedUnknown
}

// PhysicalInterfaceState holds the state container for port discovery.
type PhysicalInterfaceState struct {
	Type InterfaceType `json:"type,omitempty"`
}

// PhysicalInterfaceEthernet holds ethernet state for port speed discovery.
type PhysicalInterfaceEthernet struct {
	State *PhysicalEthernetState `json:"state,omitempty"`
}

// PhysicalEthernetState holds ethernet state fields.
type PhysicalEthernetState struct {
	PortSpeed EthernetSpeed `json:"port-speed,omitempty"`
}

// EthernetSpeed represents the YANG identity for ethernet port speed.
type EthernetSpeed string

// Short returns the speed without the module prefix (e.g. "25GB").
func (s EthernetSpeed) Short() string {
	return strings.TrimPrefix(string(s), "openconfig-if-ethernet:SPEED_")
}

const (
	EthernetSpeed10MB    EthernetSpeed = "openconfig-if-ethernet:SPEED_10MB"
	EthernetSpeed100MB   EthernetSpeed = "openconfig-if-ethernet:SPEED_100MB"
	EthernetSpeed1GB     EthernetSpeed = "openconfig-if-ethernet:SPEED_1GB"
	EthernetSpeed2500MB  EthernetSpeed = "openconfig-if-ethernet:SPEED_2500MB"
	EthernetSpeed5GB     EthernetSpeed = "openconfig-if-ethernet:SPEED_5GB"
	EthernetSpeed10GB    EthernetSpeed = "openconfig-if-ethernet:SPEED_10GB"
	EthernetSpeed25GB    EthernetSpeed = "openconfig-if-ethernet:SPEED_25GB"
	EthernetSpeed40GB    EthernetSpeed = "openconfig-if-ethernet:SPEED_40GB"
	EthernetSpeed50GB    EthernetSpeed = "openconfig-if-ethernet:SPEED_50GB"
	EthernetSpeed100GB   EthernetSpeed = "openconfig-if-ethernet:SPEED_100GB"
	EthernetSpeed200GB   EthernetSpeed = "openconfig-if-ethernet:SPEED_200GB"
	EthernetSpeed400GB   EthernetSpeed = "openconfig-if-ethernet:SPEED_400GB"
	EthernetSpeed600GB   EthernetSpeed = "openconfig-if-ethernet:SPEED_600GB"
	EthernetSpeed800GB   EthernetSpeed = "openconfig-if-ethernet:SPEED_800GB"
	EthernetSpeed1600GB  EthernetSpeed = "openconfig-if-ethernet:SPEED_1600GB"
	EthernetSpeedUnknown EthernetSpeed = "openconfig-if-ethernet:SPEED_UNKNOWN"
)
