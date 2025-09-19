// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"sort"
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

// InvRange inverts a string representation of identifiers (typically VLAN IDs) into a slice of uint16.
func InvRange(r string) ([]uint16, error) {
	if r == "" {
		return nil, nil
	}
	var result []uint16
	for part := range strings.SplitSeq(r, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range: %q", part)
			}
			start, err := strconv.ParseUint(bounds[0], 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid start in range %q: %w", part, err)
			}
			end, err := strconv.ParseUint(bounds[1], 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid end in range %q: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("start greater than end in range %q", part)
			}
			for v := start; v <= end; v++ {
				result = append(result, uint16(v))
			}
		} else {
			val, err := strconv.ParseUint(part, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q: %w", part, err)
			}
			result = append(result, uint16(val))
		}
	}
	return result, nil
}

func (p *PhysIf) FromYGOT(ctx context.Context, client gnmiext.Client) error {
	i := &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{}

	if err := client.Get(ctx, "System/intf-items/phys-items/PhysIf-list[id="+p.name+"]", i); err != nil {
		return fmt.Errorf("physif: interface %s does not exist on device: %w", p.name, err)
	}

	if i.Descr != nil {
		p.description = *i.Descr
	}
	p.adminSt = i.AdminSt == nxos.Cisco_NX_OSDevice_L1_AdminSt_up
	if i.Mtu != nil {
		p.mtu = *i.Mtu
	}
	// TDn is of the form "System/inst-items/Inst-list[name=VRF_NAME]"
	if i.GetRtvrfMbrItems() != nil && i.GetRtvrfMbrItems().TDn != nil {
		re := regexp.MustCompile(`\[name=([^\]]+)\]`)
		matches := re.FindStringSubmatch(*i.GetRtvrfMbrItems().TDn)
		if len(matches) == 2 {
			p.vrf = matches[1]
		}
	}

	switch i.Layer {
	case nxos.Cisco_NX_OSDevice_L1_Layer_Layer2:
		err := p.fromYGOTL2(ctx, client, i)
		if err != nil {
			return fmt.Errorf("physif: FromYGOT failed to parse L2 configuration %w", err)
		}
	case nxos.Cisco_NX_OSDevice_L1_Layer_Layer3:
		err := p.fromYGOTL3(ctx, client, i)
		if err != nil {
			return fmt.Errorf("physif: FromYGOT failed to parse L3 configuration %w", err)
		}
	}

	return nil
}

func (p *PhysIf) fromYGOTL2(_ context.Context, client gnmiext.Client, i *nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList) error {
	// base config
	p.l2 = &L2Config{}
	switch i.Mode {
	case nxos.Cisco_NX_OSDevice_L1_Mode_access:
		p.l2.switchPort = SwitchPortModeAccess
	case nxos.Cisco_NX_OSDevice_L1_Mode_trunk:
		p.l2.switchPort = SwitchPortModeTrunk
	default:
		return errors.New("physif: unexpected switchport mode for L2 interface")
	}
	if i.AccessVlan != nil {
		vlanStr := strings.TrimPrefix(*i.AccessVlan, "vlan-")
		vlanID, err := strconv.ParseUint(vlanStr, 10, 16)
		if err != nil {
			return fmt.Errorf("physif: failed to parse access VLAN ID: %w", err)
		}
		p.l2.accessVlan = uint16(vlanID)
	}
	if i.TrunkVlans != nil {
		vlans, err := InvRange(*i.TrunkVlans)
		if err != nil {
			return fmt.Errorf("physif: failed to parse trunk allowed VLANs: %w", err)
		}
		slices.Sort(vlans)
		p.l2.allowedVlans = vlans
	}
	if i.NativeVlan != nil {
		vlanStr := strings.TrimPrefix(*i.NativeVlan, "vlan-")
		vlanID, err := strconv.ParseUint(vlanStr, 10, 16)
		if err != nil {
			return fmt.Errorf("physif: failed to parse native VLAN ID: %w", err)
		}
		p.l2.nativeVlan = uint16(vlanID)
	}
	// spanning tree
	p.l2.spanningTree = SpanningTreeModeUnset
	s := nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{}
	err := client.Get(context.Background(), "System/stp-items/inst-items/if-items/If-list[id="+p.name+"]", &s)
	if err != nil {
		return fmt.Errorf("physif: failed to get spanning tree config for interface %s: %w", p.name, err)
	}
	switch s.Mode {
	case nxos.Cisco_NX_OSDevice_Stp_IfMode_edge:
		p.l2.spanningTree = SpanningTreeModeEdge
	case nxos.Cisco_NX_OSDevice_Stp_IfMode_network:
		p.l2.spanningTree = SpanningTreeModeNetwork
	case nxos.Cisco_NX_OSDevice_Stp_IfMode_trunk:
		p.l2.spanningTree = SpanningTreeModeTrunk
	}
	return nil
}

// fromYGOTL3 populates the L3 configuration of the physical interface from the YGOT model. VRF must be
// already be correctly set on the PhysIf struct.
func (p *PhysIf) fromYGOTL3(_ context.Context, client gnmiext.Client, i *nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList) error {
	p.l3 = &L3Config{}
	p.l3.medium = L3MediumTypeUnset
	switch i.Medium {
	case nxos.Cisco_NX_OSDevice_L1_Medium_broadcast:
		p.l3.medium = L3MediumTypeBroadcast
	case nxos.Cisco_NX_OSDevice_L1_Medium_p2p:
		p.l3.medium = L3MediumTypeP2P
	}
	// get addressing mode and addresses
	vrfName := "default"
	if p.vrf != "" {
		vrfName = p.vrf
	}
	a := &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{}
	err := client.Get(context.Background(), "System/ipv4-items/inst-items/dom-items/Dom-list[name="+vrfName+"]/if-items/If-list[id="+p.name+"]", a)
	if err != nil {
		return errors.New("physif: failed to get IPv4 config for L3 physical interface")
	}
	if a.Unnumbered != nil {
		p.l3.addressingMode = AddressingModeUnnumbered
		p.l3.unnumberedLoopback = *a.Unnumbered
		return nil
	}
	if a.AddrItems == nil || len(a.AddrItems.AddrList) == 0 {
		return nil
	}
	p.l3.addressingMode = AddressingModeNumbered
	for _, addr := range a.GetOrCreateAddrItems().AddrList {
		a, err := netip.ParsePrefix(*addr.Addr)
		if err != nil {
			return fmt.Errorf("physif: failed to parse IPv4 address: %w", err)
		}
		p.l3.prefixesIPv4 = append(p.l3.prefixesIPv4, a)
	}
	// IPv6 addresses
	v6 := &nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList{}
	err = client.Get(context.Background(), "System/ipv6-items/inst-items/dom-items/Dom-list[name="+vrfName+"]/if-items/If-list[id="+p.name+"]", v6)
	if err != nil {
		return errors.New("physif: failed to get IPv6 config for L3 physical interface")
	}
	if v6.AddrItems == nil || len(v6.AddrItems.AddrList) == 0 {
		return nil
	}
	for _, addr := range v6.GetOrCreateAddrItems().AddrList {
		a, err := netip.ParsePrefix(*addr.Addr)
		if err != nil {
			return fmt.Errorf("physif: failed to parse IPv6 address: %w", err)
		}
		p.l3.prefixesIPv6 = append(p.l3.prefixesIPv6, a)
	}
	return nil
}

func sortedCopyPrefix(prefixes []netip.Prefix) []netip.Prefix {
	result := slices.Clone(prefixes)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Addr() != result[j].Addr() {
			return result[i].Addr().Less(result[j].Addr())
		}
		return result[i].Bits() < result[j].Bits()
	})
	return result
}

func sortedCopyUint16(s []uint16) []uint16 {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func (p *PhysIf) Equals(other gnmiext.DeviceConf) (bool, error) {
	o, ok := other.(*PhysIf)
	if !ok {
		return false, fmt.Errorf("type mismatch: expected *PhysIf, got %T", other)
	}
	if p.name != o.name ||
		p.description != o.description ||
		p.adminSt != o.adminSt ||
		p.mtu != o.mtu ||
		p.vrf != o.vrf {
		return false, nil
	}
	// Compare L2 configs (implement L2Config.Equals if needed)
	if p.l2 == nil != (o.l2 == nil) {
		return false, nil
	}
	if p.l2 != nil && o.l2 != nil {
		if p.l2.switchPort != o.l2.switchPort ||
			p.l2.spanningTree != o.l2.spanningTree ||
			p.l2.accessVlan != o.l2.accessVlan ||
			p.l2.nativeVlan != o.l2.nativeVlan ||
			!slices.Equal(sortedCopyUint16(p.l2.allowedVlans), sortedCopyUint16(o.l2.allowedVlans)) {
			return false, nil
		}
	}

	// Compare L3 configs, ignoring slice order
	if (p.l3 == nil) != (o.l3 == nil) {
		return false, nil
	}
	if p.l3 != nil && o.l3 != nil {
		if p.l3.medium != o.l3.medium ||
			p.l3.addressingMode != o.l3.addressingMode ||
			p.l3.unnumberedLoopback != o.l3.unnumberedLoopback {
			return false, nil
		}
		pv4 := sortedCopyPrefix(p.l3.prefixesIPv4)
		ov4 := sortedCopyPrefix(o.l3.prefixesIPv4)
		pv6 := sortedCopyPrefix(p.l3.prefixesIPv6)
		ov6 := sortedCopyPrefix(o.l3.prefixesIPv6)
		if !slices.Equal(pv4, ov4) || !slices.Equal(pv6, ov6) {
			return false, nil
		}
	}
	return true, nil
}

func (p *PhysIf) String() string {
	result := fmt.Sprintf("PhysIf(name=%q, description=%q, adminSt=%t, mtu=%d, vrf=%q", p.name, p.description, p.adminSt, p.mtu, p.vrf)
	if p.l2 != nil {
		result += ", l2=" + p.l2.String()
	}
	if p.l3 != nil {
		result += ", l3=" + p.l3.String()
	}
	result += ")"
	return result
}

func (c *L2Config) String() string {
	return fmt.Sprintf(
		"L2Config(switchPort=%s, spanningTree=%d, accessVlan=%d, nativeVlan=%d, allowedVlans=%v)",
		c.switchPort.String(), c.spanningTree, c.accessVlan, c.nativeVlan, c.allowedVlans,
	)
}

// String returns the string representation of the SwitchPortMode.
func (m SwitchPortMode) String() string {
	switch m {
	case SwitchPortModeAccess:
		return "access"
	case SwitchPortModeTrunk:
		return "trunk"
	default:
		return "unknown"
	}
}

func (c *L3Config) String() string {
	return fmt.Sprintf(
		"L3Config(medium=%d, addressingMode=%d, unnumberedLoopback=%q, prefixesIPv4=%v, prefixesIPv6=%v)",
		c.medium, c.addressingMode, c.unnumberedLoopback, c.prefixesIPv4, c.prefixesIPv6,
	)
}
