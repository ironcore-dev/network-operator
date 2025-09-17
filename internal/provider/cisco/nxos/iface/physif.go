// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

// PhysIf represents a physical interface on a Cisco Nexus device and implements the gnmiext.DeviceConf interface
// to enable configuration via the gnmiext package.
var _ gnmiext.DeviceConf = (*PhysIf)(nil)

type PhysIf struct {
	name        string
	description string
	adminSt     bool
	mtu         uint32
	// Layer 2 properties, e.g., switchport mode, spanning tree, etc.
	l2 *L2Config
	// Layer 3 properties, e.g., IP address
	l3 *L3Config
	// vrf setting resides on the physical interface yang subtree
	vrf string
}

type PhysIfOption func(*PhysIf) error

// NewPhysicalInterface creates a new physical interface with the given name and description.
//   - Name must follow the NX-OS naming convention, e.g., "Ethernet1/1" or "eth1/1" (case insensitive).
//   - The interface will be configured admin state set to `up`.
//   - If both L2 and L3 configurations options are supplied, only the last one will be applied.
func NewPhysicalInterface(name string, opts ...PhysIfOption) (*PhysIf, error) {
	shortName, err := ShortNamePhysicalInterface(name)
	if err != nil {
		return nil, err
	}
	p := &PhysIf{
		name:    shortName,
		adminSt: true,
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// WithDescription sets a description on the physical interface.
func WithDescription(descr string) PhysIfOption {
	return func(p *PhysIf) error {
		if descr == "" {
			return errors.New("physif: description must not be empty")
		}
		p.description = descr
		return nil
	}
}

// WithPhysIfMTU sets the MTU for the physical interface.
func WithPhysIfMTU(mtu uint32) PhysIfOption {
	return func(p *PhysIf) error {
		if mtu > 9216 || mtu < 576 {
			return errors.New("physif: MTU must be between 576 and 9216")
		}
		p.mtu = mtu
		return nil
	}
}

// WithPhysIfL2 sets a Layer 2 configuration for the physical interface.
func WithPhysIfL2(c *L2Config) PhysIfOption {
	return func(p *PhysIf) error {
		if c == nil {
			return errors.New("physif: l2 configuration cannot be nil")
		}
		p.l3 = nil // PhysIf cannot have both L2 and L3 configuration
		p.vrf = "" // reset VRF for L2 configuration
		p.l2 = c
		return nil
	}
}

// WithPhysIfL3 sets a Layer 3 configuration for the physical interface.
func WithPhysIfL3(c *L3Config) PhysIfOption {
	return func(p *PhysIf) error {
		if c == nil {
			return errors.New("physif: l3 configuration cannot be nil")
		}
		p.l2 = nil // PhysIf cannot have both L2 and L3 configuration
		p.l3 = c
		return nil
	}
}

func WithPhysIfVRF(vrf string) PhysIfOption {
	return func(p *PhysIf) error {
		if vrf == "" {
			return errors.New("physif: VRF name cannot be empty")
		}
		if p.l2 != nil {
			return errors.New("physif: cannot set VRF for a physical interface with L2 configuration")
		}
		p.vrf = vrf
		return nil
	}
}

func WithPhysIfAdminState(adminSt bool) PhysIfOption {
	return func(p *PhysIf) error {
		p.adminSt = adminSt
		return nil
	}
}

// ToYGOT returns a slice of updates for the physical interface:
//   - the first update always replaces the entire base configuration of the physical interface (gnmiext.ReplacingUpdate)
//   - subsequent updates modify the base configuration to add L2 and L3 configurations, if applicable
//   - the last update attaches the physical interface to a port channel, if applicable
func (p *PhysIf) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	var descr *string
	if p.description != "" {
		descr = &p.description
	}

	pl := &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
		AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
		Descr:         descr,
		UserCfgdFlags: ygot.String("admin_state"),
	}
	if !p.adminSt {
		pl.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_down
	}
	if p.mtu != 0 {
		pl.UserCfgdFlags = ygot.String("admin_mtu," + *pl.UserCfgdFlags)
		pl.Mtu = ygot.Uint32(p.mtu)
	}
	if p.vrf != "" {
		pl.GetOrCreateRtvrfMbrItems().TDn = ygot.String("System/inst-items/Inst-list[name=" + p.vrf + "]")
	}

	// base config must to be in the first update
	updates := []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/intf-items/phys-items/PhysIf-list[id=" + p.name + "]",
			Value: pl,
		},
	}

	// l2 (modifies part of the base tree)
	l2Updates := p.createL2(pl)
	updates = append(updates, l2Updates...)

	// l3 (modifies part of the base tree)
	l3Updates, err := p.createL3(pl)
	if err != nil {
		return nil, fmt.Errorf("physif: fail to create ygot objects for L3 config %w", err)
	}
	updates = append(updates, l3Updates...)

	return updates, nil
}

// createL2 performs in-place modification of the physical interface to enable the interface as an L2 switchport, and a
// specific spanning tree mode (if applicable).
func (p *PhysIf) createL2(pl *nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList) []gnmiext.Update {
	if p.l2 != nil {
		pl.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_UNSET
		if p.l2.switchPort == SwitchPortModeAccess || p.l2.switchPort == SwitchPortModeTrunk {
			pl.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer2
			pl.UserCfgdFlags = ygot.String("admin_layer," + *pl.UserCfgdFlags)
			switch p.l2.switchPort {
			case SwitchPortModeAccess:
				pl.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_access
				if p.l2.accessVlan != 0 {
					pl.AccessVlan = ygot.String("vlan-" + strconv.FormatUint(uint64(p.l2.accessVlan), 10))
				}
			case SwitchPortModeTrunk:
				pl.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
				if len(p.l2.allowedVlans) != 0 {
					pl.TrunkVlans = ygot.String(Range(p.l2.allowedVlans))
				}
				if p.l2.nativeVlan != 0 {
					pl.NativeVlan = ygot.String("vlan-" + strconv.FormatUint(uint64(p.l2.nativeVlan), 10))
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

// createL3 performs in-place modification of the physical interface to enable the interface as an L3 interface, and generates
// the necessary updates related to the L3 configuration of the interface.
func (p *PhysIf) createL3(pl *nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList) ([]gnmiext.Update, error) {
	if p.l3 != nil {
		pl.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer3
		pl.UserCfgdFlags = ygot.String("admin_layer," + *pl.UserCfgdFlags)
		switch p.l3.medium {
		case L3MediumTypeBroadcast:
			pl.Medium = nxos.Cisco_NX_OSDevice_L1_Medium_broadcast
		case L3MediumTypeP2P:
			pl.Medium = nxos.Cisco_NX_OSDevice_L1_Medium_p2p
		default:
			pl.Medium = nxos.Cisco_NX_OSDevice_L1_Medium_UNSET
		}
		vrfName := p.vrf
		if vrfName == "" {
			vrfName = "default"
		}
		return p.l3.ToYGOT(p.name, vrfName)
	}
	return nil, nil
}

// Reset clears config of the physical interface as well as L2, L3 options.
//   - In this Cisco Nexus version devices clean up parts of the  models that are related but in different paths of the YANG tree
//   - The same occurs for the L2 and L3 configurations options, except for the spanning tree configuration, which is not automatically reset.
func (p *PhysIf) Reset(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	updates := []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/intf-items/phys-items/PhysIf-list[id=" + p.name + "]",
			Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
		},
	}

	exists, err := client.Exists(ctx, "System/stp-items/inst-items/if-items/If-list[id="+p.name+"]")
	if err != nil {
		return nil, err
	}

	if exists {
		updates = slices.Insert(updates, 0, gnmiext.Update(gnmiext.ReplacingUpdate{
			XPath: "System/stp-items/inst-items/if-items/If-list[id=" + p.name + "]",
			Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
		}))
	}

	return updates, nil
}

// Range provides a string representation of identifiers (typically VLAN IDs) that formats the range in a human-readable way.
// Consecutive IDs are represented as a range (e.g., "10-12"), while single IDs are shown individually (e.g., "15").
// All values are joined in a comma-separated list of ranges and individual IDs, e.g. "10-12,15,20-22".
func Range(r []uint16) string {
	if len(r) == 0 {
		return ""
	}

	slices.Sort(r)
	var ranges []string
	start, curr := r[0], r[0]
	for _, id := range r[1:] {
		if id == curr+1 {
			curr = id
			continue
		}
		if curr != start {
			ranges = append(ranges, fmt.Sprintf("%d-%d", start, curr))
		} else {
			ranges = append(ranges, strconv.FormatInt(int64(start), 10))
		}
		start, curr = id, id
	}
	if curr != start {
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, curr))
	} else {
		ranges = append(ranges, strconv.FormatInt(int64(start), 10))
	}

	return strings.Join(ranges, ",")
}
