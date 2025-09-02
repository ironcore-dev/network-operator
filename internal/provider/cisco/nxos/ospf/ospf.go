// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package ospf

import (
	"context"
	"errors"
	"net/netip"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*OSPF)(nil)

// OSPF represents an OSPF process for a Cisco NX-OS device. New OSPF processes should be created using the NewOSPF function.
type OSPF struct {
	adminSt bool
	// name of the OSPF process, e.g., `IPN`
	name string
	// ID is the router ID of the OSPF process, must be a valid IPv4 address and
	// must exist on a configured interface in the system.
	id netip.Addr
	// propagateDefaultRoute is equivalent to the CLI command `default-information originate`
	progateDefaultRoute bool
	// redistributionConfigs is a list of redistribution configurations for the OSPF process.
	redistributionConfigs []RedistributionConfig
	// logLevel is the logging level for OSPF adjacency changes. By default "none"
	logLevel LogLevel
	// distance is the adminitrative distance value (1-255) for OSPF routes. Cisco's default is 110.
	distance uint8
	// referenceBandwidthMbps is the reference bandwidth in Mbps used for OSPF calculations. By default Cisco NX-OS
	// assigns a cost that is the configured reference bandwidth divided by the interface bandwidth. The
	// the reference bandwidth in these devices is 40 Gbps. Must be between 1 and 999999 Mbps.
	referenceBandwidthMbps uint32
	// maxLSA is the maximum number of non self-generated LSAs (min 1)
	maxLSA uint32
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=LogLevel
type LogLevel int

const (
	None LogLevel = iota
	Brief
	Detail
)

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=DistributionProtocol
type DistributionProtocol int

const (
	DistributionProtocolDirect DistributionProtocol = iota + 1
	DistributionProtocolStatic
)

// RedistributionConfig represents a redistribution configuration of a route map through a specific protocol.
type RedistributionConfig struct {
	// Protocol to redistribute, e.g., `direct`
	protocol DistributionProtocol
	// Route map to apply, e.g., `REDIST-ALL`
	routeMapName string
}

// NewOSPF creates a new OSPFv3 instance with the given name and ID. The ID must be a valid IPv4 address and must
// exist on a configured interface in the system.
func NewOSPF(name, id string, opts ...Option) (*OSPF, error) {
	if name == "" {
		return nil, errors.New("OSPF name cannot be empty")
	}
	addr, err := netip.ParseAddr(id)
	if err != nil || !addr.Is4() {
		return nil, errors.New("OSPF ID must be a valid IPv4 address")
	}
	o := &OSPF{
		name:    name,
		id:      addr,
		adminSt: true, // default to enabled
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

type Option func(*OSPF) error

func WithAdminState(enabled bool) Option {
	return func(o *OSPF) error {
		o.adminSt = enabled
		return nil
	}
}

func WithDistance(distance uint8) Option {
	return func(o *OSPF) error {
		if distance < 1 {
			return errors.New("distance must be between 1 and 255")
		}
		o.distance = distance
		return nil
	}
}

func WithReferenceBandwidthMbps(mbps uint32) Option {
	return func(o *OSPF) error {
		if mbps < 1 || mbps > 999999 {
			return errors.New("reference bandwidth must be between 1 and 999999 Mbps")
		}
		o.referenceBandwidthMbps = mbps
		return nil
	}
}

func WithMaxLSA(maxLSA uint32) Option {
	return func(o *OSPF) error {
		if maxLSA < 1 {
			return errors.New("maxLSA must be at least 1")
		}
		o.maxLSA = maxLSA
		return nil
	}
}

// WithLogLevel sets the logging level for OSPF adjacency changes. Default is "none". If level
// is set to "brief", it will log brief adjacency changes (equivalent to "log-adjacency-changes").
// If set to "detail", it will log detailed adjacency changes (equivalent to "log-adjacency-changes detail").
func WithLogLevel(level LogLevel) Option {
	return func(o *OSPF) error {
		switch level {
		case None, Brief, Detail:
			o.logLevel = level
			return nil
		default:
			return errors.New("unsupported OSPF log level: " + level.String())
		}
	}
}

// WithDefaultRoutePropagation enables the propagation of the default route in OSPF. With "true" we do the equivalent
// to the CLI command `default-information originate`.
func WithDefaultRoutePropagation(propagate bool) Option {
	return func(o *OSPF) error {
		o.progateDefaultRoute = propagate
		return nil
	}
}

func WithRedistributionConfig(distributionProtocol DistributionProtocol, routeMapName string) Option {
	return func(o *OSPF) error {
		if routeMapName == "" {
			return errors.New("route map name cannot be empty")
		}
		switch distributionProtocol {
		case DistributionProtocolDirect, DistributionProtocolStatic:
			// Check for duplicate
			for _, cfg := range o.redistributionConfigs {
				if cfg.protocol == distributionProtocol && cfg.routeMapName == routeMapName {
					return errors.New("duplicate redistribution config: " + distributionProtocol.String() + " with route map " + routeMapName)
				}
			}
			o.redistributionConfigs = append(o.redistributionConfigs, RedistributionConfig{
				protocol:     distributionProtocol,
				routeMapName: routeMapName,
			})
			return nil
		default:
			return errors.New("unsupported redistribution protocol: " + distributionProtocol.String())
		}
	}
}

// ToYGOT converts the OSPF configuration to a YANG model representation suitable for gNMI operations.
// It returns a slice of gnmiext.Update, where: 1) the first update enables the OSPF feature in the system,
// and 2) the second update contains the OSPF process configuration.
func (o *OSPF) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	val := nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList{
		Name: ygot.String(o.name),
	}
	val.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	if !o.adminSt {
		val.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled
	}

	domList := val.GetOrCreateDomItems().GetOrCreateDomList("default")
	domList.RtrId = ygot.String(o.id.String())

	if o.distance != 0 {
		domList.Dist = ygot.Uint8(o.distance)
	}

	if o.progateDefaultRoute {
		domList.DefrtleakItems = &nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_DefrtleakItems{
			Always: nxos.Cisco_NX_OSDevice_Ospf_Always_no,
		}
	}

	if o.maxLSA != 0 {
		domList.MaxlsapItems = &nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_MaxlsapItems{
			Action: nxos.Cisco_NX_OSDevice_Ospf_MaxLsaAct_reject,
			MaxLsa: ygot.Uint32(o.maxLSA),
		}
	}

	if o.referenceBandwidthMbps != 0 {
		domList.BwRef = ygot.Uint32(o.referenceBandwidthMbps)
		domList.BwRefUnit = nxos.Cisco_NX_OSDevice_Ospf_BwRefUnit_mbps
	}

	switch o.logLevel {
	case Brief:
		domList.AdjChangeLogLevel = nxos.Cisco_NX_OSDevice_Ospf_AdjChangeLogLevel_brief
	case Detail:
		domList.AdjChangeLogLevel = nxos.Cisco_NX_OSDevice_Ospf_AdjChangeLogLevel_detail
	default:
		domList.AdjChangeLogLevel = nxos.Cisco_NX_OSDevice_Ospf_AdjChangeLogLevel_none
	}

	for _, cfg := range o.redistributionConfigs {
		switch cfg.protocol {
		case DistributionProtocolStatic:
			interLeakP := domList.GetOrCreateInterleakItems().GetOrCreateInterLeakPList(nxos.Cisco_NX_OSDevice_Rtleak_Proto_static, "none", "none")
			interLeakP.RtMap = ygot.String(cfg.routeMapName)
		case DistributionProtocolDirect:
			interLeakP := domList.GetOrCreateInterleakItems().GetOrCreateInterLeakPList(nxos.Cisco_NX_OSDevice_Rtleak_Proto_direct, "none", "none")
			interLeakP.RtMap = ygot.String(cfg.routeMapName)
		default:
			return nil, errors.New("unsupported redistribution protocol: " + cfg.protocol.String())
		}
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/ospf-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/ospf-items/inst-items/Inst-list",
			Value: &val,
		},
	}, nil
}

// Reset removes the OSPF process with the given name from the device.
func (o *OSPF) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/ospf-items/inst-items/Inst-list[name=" + o.name + "]",
		},
	}, nil
}
