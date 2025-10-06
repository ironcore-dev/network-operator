// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*Loopback)(nil)
	_ gnmiext.Configurable = (*PhysIf)(nil)
	_ gnmiext.Defaultable  = (*PhysIf)(nil)
	_ gnmiext.Configurable = (*VrfMember)(nil)
	_ gnmiext.Configurable = (*SpanningTree)(nil)
	_ gnmiext.Configurable = (*AddrItem)(nil)
)

// Loopback represents a loopback interface on a NX-OS device.
type Loopback struct {
	AdminSt       AdminSt2   `json:"adminSt"`
	Descr         string     `json:"descr"`
	ID            string     `json:"id"`
	RtvrfMbrItems *VrfMember `json:"rtvrfMbr-items,omitempty"`
}

func (l *Loopback) IsListItem() {}

func (l *Loopback) XPath() string {
	return "System/intf-items/lb-items/LbRtdIf-list[id=" + l.ID + "]"
}

const (
	DefaultVLAN      = "vlan-1"
	DefaultVLANRange = "1-4094"
	DefaultMTU       = 1500
)

// PhysIf represents a physical (ethernet) interface on a NX-OS device.
type PhysIf struct {
	AccessVlan    string         `json:"accessVlan"`
	AdminSt       AdminSt2       `json:"adminSt"`
	Descr         string         `json:"descr"`
	ID            string         `json:"id"`
	Layer         Layer          `json:"layer"`
	MTU           int32          `json:"mtu,omitempty"`
	Medium        Medium         `json:"medium"`
	Mode          SwitchportMode `json:"mode"`
	NativeVlan    string         `json:"nativeVlan"`
	TrunkVlans    string         `json:"trunkVlans"`
	UserCfgdFlags string         `json:"userCfgdFlags"`
	RtvrfMbrItems *VrfMember     `json:"rtvrfMbr-items,omitempty"`
}

func (p *PhysIf) IsListItem() {}

func (p *PhysIf) XPath() string {
	return "System/intf-items/phys-items/PhysIf-list[id=" + p.ID + "]"
}

func (p *PhysIf) Validate() error {
	if p.MTU != 0 && (p.MTU < 576 || p.MTU > 9216) {
		return fmt.Errorf("physical interface: mtu must be between 576 and 9216, got %d", p.MTU)
	}
	return nil
}

func (p *PhysIf) Default() {
	p.AccessVlan = DefaultVLAN
	p.AdminSt = AdminStDown
	p.Layer = Layer2
	p.MTU = DefaultMTU
	p.Medium = MediumBroadcast
	p.Mode = SwitchportModeAccess
	p.NativeVlan = DefaultVLAN
	p.TrunkVlans = DefaultVLANRange
	p.UserCfgdFlags = "admin_state"
}

// VrfMember represents a VRF associtation for an interface.
type VrfMember struct {
	TDn    string `json:"tDn"`
	IfName string `json:"-"`
}

func (v *VrfMember) XPath() string {
	if loopbackRe.MatchString(v.IfName) {
		return "System/intf-items/lb-items/LbRtdIf-list[id=" + v.IfName + "]/rtvrfMbr-items"
	}
	return "System/intf-items/phys-items/PhysIf-list[id=" + v.IfName + "]/rtvrfMbr-items"
}

func NewVrfMember(ifName, vrfName string) *VrfMember {
	return &VrfMember{
		TDn:    fmt.Sprintf("/System/inst-items/Inst-list[name='%s']", vrfName),
		IfName: ifName,
	}
}

// SpanningTree represents the spanning tree configuration for an interface.
type SpanningTree struct {
	AdminSt AdminSt          `json:"adminSt"`
	Mode    SpanningTreeMode `json:"mode"`
	IfName  string           `json:"-"`
}

func (s *SpanningTree) IsListItem() {}

func (s *SpanningTree) XPath() string {
	return "System/stp-items/inst-items/if-items/If-list[id=" + s.IfName + "]"
}

// PortChannel represents a port-channel (LAG) interface on a NX-OS device.
type PortChannel struct {
	AccessVlan    string          `json:"accessVlan"`
	AdminSt       AdminSt2        `json:"adminSt"`
	Descr         string          `json:"descr,omitempty"`
	ID            string          `json:"id"`
	Layer         Layer           `json:"layer"`
	Mode          SwitchportMode  `json:"mode"`
	PcMode        PortChannelMode `json:"pcMode"`
	NativeVlan    string          `json:"nativeVlan"`
	TrunkVlans    string          `json:"trunkVlans"`
	UserCfgdFlags string          `json:"userCfgdFlags"`
	RsmbrIfsItems struct {
		RsMbrIfsList []*PortChannelMember `json:"RsMbrIfs-list,omitzero"`
	} `json:"rsmbrIfs-items,omitzero"`
}

type PortChannelMember struct {
	TDn string `json:"tDn"`
}

func (p *PortChannel) IsListItem() {}

func (p *PortChannel) XPath() string {
	return "System/intf-items/aggr-items/AggrIf-list[id=" + p.ID + "]"
}

// AddrItem represents the IP address configuration for an interface.
// It can hold either IPv4 or IPv6 addresses, determined by the Is6 field.
type AddrItem struct {
	ID         string `json:"id"`
	Unnumbered string `json:"unnumbered,omitempty"`
	AddrItems  struct {
		AddrList []*IntfAddr `json:"Addr-list,omitzero"`
	} `json:"addr-items,omitzero"`
	// Is6 indicates whether the addresses are IPv6 (true) or IPv4 (false).
	// This field is not serialized to JSON and is only used internally to
	// determine the correct XPath for the address.
	Is6 bool `json:"-"`
}

func (a *AddrItem) IsListItem() {}

func (a *AddrItem) XPath() string {
	if a.Is6 {
		return "System/ipv6-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + a.ID + "]"
	}
	return "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + a.ID + "]"
}

type IntfAddr struct {
	Addr string `json:"addr"`
	Pref int    `json:"pref"`
	Tag  int    `json:"tag"`
	Type string `json:"type"`
}

// Range provides a string representation of identifiers (typically VLAN IDs) that formats the range in a human-readable way.
// Consecutive IDs are represented as a range (e.g., "10-12"), while single IDs are shown individually (e.g., "15").
// All values are joined in a comma-separated list of ranges and individual IDs, e.g. "10-12,15,20-22".
func Range(r []int32) string {
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

// Exists checks if all provided interface names exist on the device.
func Exists(ctx context.Context, client *gnmiext.Client, names ...string) (bool, error) {
	if len(names) == 0 {
		return false, errors.New("at least one interface name must be provided")
	}
	conf := make([]gnmiext.Configurable, 0, len(names))
	for _, name := range names {
		if name == "" {
			return false, errors.New("interface name must not be empty")
		}
		if mgmtRe.MatchString(name) {
			// mgmt0 is always present
			continue
		}
		var c gnmiext.Configurable
		if matches := ethernetRe.FindStringSubmatch(name); matches != nil {
			c = &PhysIf{ID: "eth" + matches[2]}
		}
		if matches := loopbackRe.FindStringSubmatch(name); matches != nil {
			c = &Loopback{ID: "lo" + matches[2]}
		}
		if matches := portchannelRe.FindStringSubmatch(name); matches != nil {
			c = &PortChannel{ID: "po" + matches[2]}
		}
		if c == nil {
			return false, fmt.Errorf("unsupported interface format %q, expected one of: %s, %s, %s, %s", name, mgmtRe.String(), ethernetRe.String(), loopbackRe.String(), portchannelRe.String())
		}
		conf = append(conf, c)
	}
	err := client.GetConfig(ctx, conf...)
	if errors.Is(err, gnmiext.ErrNil) {
		return false, nil
	}
	return err == nil, err
}

type Layer string

const (
	Layer2 Layer = "Layer2"
	Layer3 Layer = "Layer3"
)

type Medium string

const (
	MediumBroadcast    Medium = "broadcast"
	MediumPointToPoint Medium = "p2p"
)

type SwitchportMode string

const (
	SwitchportModeAccess SwitchportMode = "access"
	SwitchportModeTrunk  SwitchportMode = "trunk"
)

type SpanningTreeMode string

const (
	SpanningTreeModeEdge    SpanningTreeMode = "edge"
	SpanningTreeModeNetwork SpanningTreeMode = "network"
	SpanningTreeModeTrunk   SpanningTreeMode = "trunk"
)

type PortChannelMode string

const (
	PortChannelModeActive  PortChannelMode = "active"
	PortChannelModePassive PortChannelMode = "passive"
)
