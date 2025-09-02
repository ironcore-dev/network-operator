// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iface

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*PortChannel)(nil)

type PortChannel struct {
	name        string
	description string
	physIfs     map[string]struct{}
	// layer 2 properties, e.g., switchport mode, spanning tree, etc.
	l2 *L2Config
}

type PortChannelOption func(*PortChannel) error

// NewPortChannel creates a new port-channel interface with the given name.  Name must follow the NX-OS
// naming convention, e.g., "port-channel10", "po10". Valid range for port-channel number is 1-4096.
func NewPortChannel(name string, opts ...PortChannelOption) (*PortChannel, error) {
	shortName, err := ShortNamePortChannel(name)
	if err != nil {
		return nil, err
	}
	pcNum, err := strconv.Atoi(shortName[2:])
	if err != nil {
		return nil, fmt.Errorf("iface: invalid port-channel number in name %q: %w", name, err)
	}
	if pcNum < 1 || pcNum > 4096 {
		return nil, errors.New("iface: port-channel number must be between 1 and 4096")
	}
	p := &PortChannel{
		name:    shortName,
		physIfs: make(map[string]struct{}),
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func WithPortChannelDescription(descr string) PortChannelOption {
	return func(p *PortChannel) error {
		if descr == "" {
			return errors.New("iface: portchannel description cannot be empty")
		}
		p.description = descr
		return nil
	}
}

// WithPhysicalInterface adds a physical interface as a member to the port-channel. The interface
// name must follow the convention for physical interfaces, e.g., "Ethernet 1/1" or "eth1/1"
// (case insensitive). There are no checks performed here to ensure that the physical interface
// configuration is compatible with the port-channel (see ToYGOT for more details).
func WithPhysicalInterface(iface string) PortChannelOption {
	return func(p *PortChannel) error {
		shortName, err := ShortNamePhysicalInterface(iface)
		if err != nil {
			return err
		}
		p.physIfs[shortName] = struct{}{}
		return nil
	}
}

// WithPortChannelL2 sets a Layer 2 configuration for the physical interface.
func WithPortChannelL2(c *L2Config) PortChannelOption {
	return func(p *PortChannel) error {
		if c == nil {
			return errors.New("port-channel: l2 configuration cannot be nil")
		}
		p.l2 = c
		return nil
	}
}

// ToYGOT returns a slice of gnmiext.Updates to configure the port channel on the device.
// The first update enables LACP globally on the device. A second update configures and enables
// the port-channel. This includes adding the member physical interfaces to the port-channel.
// If a Layer 2 configuration is provided, a third update might be added.
//
// The caller must ensure that the physical interfaces added as members to the port-channel
// are configured correctly in the device (e.g., the interface mode is compatible with the port-channel
// mode, etc.). The only check being done by this function is to ensure that the physical interfaces exist
// on the device.
func (p *PortChannel) ToYGOT(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	// enable LACP globally
	updates := []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/lacp-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_LacpItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
	}

	v := &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{
		AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
		PcMode:        nxos.Cisco_NX_OSDevice_Pc_Mode_active,
		UserCfgdFlags: ygot.String("admin_state"),
	}
	if p.description != "" {
		v.Descr = &p.description
	}
	// add interfaces to the port-channel
	for i := range p.physIfs {
		exists, err := Exists(ctx, client, i)
		if err != nil {
			return nil, fmt.Errorf("port-channel: failed to check if physical interface %q exists: %w", i, err)
		}
		if !exists {
			return nil, fmt.Errorf("port-channel: the physical interface %q specified as a member of port-channel %q does not exist on the device", i, p.name)
		}
		v.GetOrCreateRsmbrIfsItems().GetOrCreateRsMbrIfsList("System/intf-items/phys-items/PhysIf-list[id=" + i + "]")
	}
	updates = append(updates, gnmiext.ReplacingUpdate{
		XPath: "System/intf-items/aggr-items/AggrIf-list[id=" + p.name + "]",
		Value: v,
	})
	l2updates := p.createL2(v)
	updates = append(updates, l2updates...)
	return updates, nil
}

// Reset returns a deleting update to remove the port channel from the device.
func (p *PortChannel) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/intf-items/aggr-items/AggrIf-list[id=" + p.name + "]",
		},
		gnmiext.DeletingUpdate{ // reset spanning tree
			XPath: "System/stp-items/inst-items/if-items/If-list[id=" + p.name + "]",
		},
	}, nil
}

// createL2 performs in-place modification of the port-channel L2 properties, including
// a spanning tree mode (if applicable).
func (p *PortChannel) createL2(a *nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList) []gnmiext.Update {
	if p.l2 != nil {
		if p.l2.switchPort == SwitchPortModeAccess || p.l2.switchPort == SwitchPortModeTrunk {
			a.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2
			a.UserCfgdFlags = ygot.String("admin_layer," + *a.UserCfgdFlags)
			switch p.l2.switchPort {
			case SwitchPortModeAccess:
				a.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_access
				if p.l2.accessVlan != 0 {
					a.AccessVlan = ygot.String("vlan-" + strconv.FormatUint(uint64(p.l2.accessVlan), 10))
				}
			case SwitchPortModeTrunk:
				a.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
				if len(p.l2.allowedVlans) != 0 {
					a.TrunkVlans = ygot.String(Range(p.l2.allowedVlans))
				}
				if p.l2.nativeVlan != 0 {
					a.NativeVlan = ygot.String("vlan-" + strconv.FormatUint(uint64(p.l2.nativeVlan), 10))
				}
			}
		}
		if p.l2.spanningTree != SpanningTreeModeUnset {
			il := nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
				AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
			}
			switch p.l2.spanningTree {
			case SpanningTreeModeEdge:
				il.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_edge
			case SpanningTreeModeNetwork:
				il.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_network
			case SpanningTreeModeTrunk:
				il.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_trunk
			default:
				il.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_UNSET
			}
			return []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=" + p.name + "]",
					Value: &il,
				},
			}
		}
	}
	return nil
}
