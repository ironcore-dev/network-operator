// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*Loopback)(nil)
	_ gnmiext.Configurable = (*LoopbackOperItems)(nil)
	_ gnmiext.Configurable = (*PhysIf)(nil)
	_ gnmiext.Defaultable  = (*PhysIf)(nil)
	_ gnmiext.Configurable = (*PhysIfOperItems)(nil)
	_ gnmiext.Configurable = (*VrfMember)(nil)
	_ gnmiext.Configurable = (*SpanningTree)(nil)
	_ gnmiext.Configurable = (*MultisiteIfTracking)(nil)
	_ gnmiext.Configurable = (*BFD)(nil)
	_ gnmiext.Configurable = (*ICMPIf)(nil)
	_ gnmiext.Configurable = (*PortChannel)(nil)
	_ gnmiext.Configurable = (*PortChannelOperItems)(nil)
	_ gnmiext.Configurable = (*SwitchVirtualInterface)(nil)
	_ gnmiext.Configurable = (*SwitchVirtualInterfaceOperItems)(nil)
	_ gnmiext.Configurable = (*AddrItem)(nil)
	_ gnmiext.Configurable = (*FabricFwdIf)(nil)
)

// Loopback represents a loopback interface on a NX-OS device.
type Loopback struct {
	AdminSt       AdminSt2   `json:"adminSt"`
	Descr         string     `json:"descr"`
	ID            string     `json:"id"`
	RtvrfMbrItems *VrfMember `json:"rtvrfMbr-items,omitempty"`
}

func (*Loopback) IsListItem() {}

func (l *Loopback) XPath() string {
	return "System/intf-items/lb-items/LbRtdIf-list[id=" + l.ID + "]"
}

type LoopbackOperItems struct {
	ID     string `json:"-"`
	OperSt OperSt `json:"operSt"`
}

func (l *LoopbackOperItems) XPath() string {
	return "System/intf-items/lb-items/LbRtdIf-list[id=" + l.ID + "]/lbrtdif-items"
}

const (
	DefaultVLAN      = "vlan-1"
	DefaultVLANRange = "1-4094"
	DefaultMTU       = 1500
)

// PhysIf represents a physical (ethernet) interface on a NX-OS device.
type PhysIf struct {
	AccessVlan    string         `json:"accessVlan"`
	AdminSt       AdminSt2       `json:"adminSt,omitempty"`
	Descr         string         `json:"descr"`
	ID            string         `json:"id"`
	Layer         Layer          `json:"layer"`
	MTU           int32          `json:"mtu,omitempty"`
	Medium        Medium         `json:"medium"`
	Mode          SwitchportMode `json:"mode"`
	NativeVlan    string         `json:"nativeVlan"`
	TrunkVlans    string         `json:"trunkVlans"`
	UserCfgdFlags UserFlags      `json:"userCfgdFlags"`
	RtvrfMbrItems *VrfMember     `json:"rtvrfMbr-items,omitempty"`
	PhysExtdItems struct {
		BufferBoost string `json:"bufferBoost,omitempty"`
	} `json:"physExtd-items,omitzero"`
}

func (*PhysIf) IsListItem() {}

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
	p.Layer = Layer2
	p.MTU = DefaultMTU
	p.Medium = MediumBroadcast
	p.Mode = SwitchportModeAccess
	p.NativeVlan = DefaultVLAN
	p.TrunkVlans = DefaultVLANRange
	p.PhysExtdItems.BufferBoost = "enable"
}

type PhysIfOperItems struct {
	ID     string `json:"-"`
	OperSt OperSt `json:"operSt"`
}

func (p *PhysIfOperItems) XPath() string {
	return "System/intf-items/phys-items/PhysIf-list[id=" + p.ID + "]/phys-items"
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
	Mode   SpanningTreeMode `json:"mode"`
	IfName string           `json:"-"`
}

func (*SpanningTree) IsListItem() {}

func (s *SpanningTree) XPath() string {
	return "System/stp-items/inst-items/if-items/If-list[id=" + s.IfName + "]"
}

func (s *SpanningTree) Default() {
	s.Mode = SpanningTreeModeDefault
}

type MultisiteIfTrackingItems struct {
	PhysIfList []struct {
		ID                  string               `json:"id"`
		MultisiteIfTracking *MultisiteIfTracking `json:"multisiteiftracking-items"`
	} `json:"PhysIf-list"`
}

func (*MultisiteIfTrackingItems) XPath() string {
	return "System/intf-items/phys-items"
}

type MultisiteIfTracking struct {
	IfName   string                  `json:"-"`
	Tracking MultisiteIfTrackingMode `json:"tracking"`
}

func (m *MultisiteIfTracking) XPath() string {
	return "System/intf-items/phys-items/PhysIf-list[id=" + m.IfName + "]/multisiteiftracking-items"
}

type BFD struct {
	ID        string  `json:"id"`
	AdminSt   AdminSt `json:"adminSt"`
	IfkaItems struct {
		DetectMult   int32 `json:"detectMult"`
		MinRxIntvlMs int64 `json:"minRxIntvl"`
		MinTxIntvlMs int64 `json:"minTxIntvl"`
	} `json:"ifka-items,omitzero"`
}

func (*BFD) IsListItem() {}

func (b *BFD) XPath() string {
	return "System/bfd-items/inst-items/if-items/If-list[id=" + b.ID + "]"
}

func (b *BFD) Validate() error {
	if b.IfkaItems.DetectMult < 1 || b.IfkaItems.DetectMult > 50 {
		return fmt.Errorf("bfd: invalid detect-mult %d: must be between 1 and 50", b.IfkaItems.DetectMult)
	}
	if b.IfkaItems.MinRxIntvlMs < 100 || b.IfkaItems.MinRxIntvlMs > 999 {
		return fmt.Errorf("bfd: invalid min-rx-intvl %d: must be between 100 and 999", b.IfkaItems.MinRxIntvlMs)
	}
	if b.IfkaItems.MinTxIntvlMs < 100 || b.IfkaItems.MinTxIntvlMs > 999 {
		return fmt.Errorf("bfd: invalid min-tx-intvl %d: must be between 100 and 999", b.IfkaItems.MinTxIntvlMs)
	}
	return nil
}

type ICMPIf struct {
	ID   string `json:"id"`
	Ctrl string `json:"ctrl"`
}

func (*ICMPIf) IsListItem() {}

func (i *ICMPIf) XPath() string {
	return "System/icmpv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + i.ID + "]"
}

// PortChannel represents a port-channel (LAG) interface on a NX-OS device.
type PortChannel struct {
	AccessVlan    string          `json:"accessVlan"`
	AdminSt       AdminSt2        `json:"adminSt"`
	Descr         string          `json:"descr,omitempty"`
	ID            string          `json:"id"`
	Layer         Layer           `json:"layer"`
	MTU           int32           `json:"mtu,omitempty"`
	Mode          SwitchportMode  `json:"mode"`
	PcMode        PortChannelMode `json:"pcMode"`
	NativeVlan    string          `json:"nativeVlan"`
	TrunkVlans    string          `json:"trunkVlans"`
	UserCfgdFlags UserFlags       `json:"userCfgdFlags"`
	RsmbrIfsItems struct {
		RsMbrIfsList gnmiext.List[string, *PortChannelMember] `json:"RsMbrIfs-list,omitzero"`
	} `json:"rsmbrIfs-items,omitzero"`
	AggrExtdItems struct {
		BufferBoost string `json:"bufferBoost,omitempty"`
	} `json:"aggrExtd-items,omitzero"`
}

type PortChannelMember struct {
	TDn   string `json:"tDn"`
	Force bool   `json:"isMbrForce,omitempty"`
}

func NewPortChannelMember(name string) *PortChannelMember {
	return &PortChannelMember{
		TDn:   fmt.Sprintf("/System/intf-items/phys-items/PhysIf-list[id='%s']", name),
		Force: false,
	}
}

func (m *PortChannelMember) Key() string { return m.TDn }

func (*PortChannel) IsListItem() {}

func (p *PortChannel) XPath() string {
	return "System/intf-items/aggr-items/AggrIf-list[id=" + p.ID + "]"
}

type PortChannelOperItems struct {
	ID         string `json:"-"`
	OperSt     OperSt `json:"operSt"`
	OperStQual string `json:"operStQual,omitempty"`
}

func (p *PortChannelOperItems) XPath() string {
	return "System/intf-items/aggr-items/AggrIf-list[id=" + p.ID + "]/aggrif-items"
}

type SwitchVirtualInterface struct {
	AdminSt       AdminSt2   `json:"adminSt"`
	Descr         string     `json:"descr"`
	ID            string     `json:"id"`
	Medium        SVIMedium  `json:"medium"`
	MTU           int32      `json:"mtu,omitempty"`
	RtvrfMbrItems *VrfMember `json:"rtvrfMbr-items,omitempty"`
	VlanID        int16      `json:"vlanId"`
}

func (*SwitchVirtualInterface) IsListItem() {}

func (s *SwitchVirtualInterface) XPath() string {
	return "System/intf-items/svi-items/If-list[id=" + s.ID + "]"
}

type SwitchVirtualInterfaceOperItems struct {
	ID     string `json:"-"`
	OperSt OperSt `json:"operSt"`
}

func (*SwitchVirtualInterfaceOperItems) IsListItem() {}

func (s *SwitchVirtualInterfaceOperItems) XPath() string {
	return "System/intf-items/svi-items/If-list[id=" + s.ID + "]"
}

type AddrList struct {
	DomList gnmiext.List[string, *AddrDom] `json:"Dom-list,omitzero"`

	// Is6 indicates whether the addresses are IPv6 (true) or IPv4 (false).
	// This field is not serialized to JSON and is only used internally to
	// determine the correct XPath for the address.
	Is6 bool `json:"-"`
}

// GetAddrItemsByInterface retrieves the address items for a given interface name.
func (a *AddrList) GetAddrItemsByInterface(name string) []*AddrItem {
	items := make([]*AddrItem, 0)
	for _, dom := range a.DomList {
		for _, item := range dom.IfItems.IfList {
			if item.ID == name {
				i := *item
				i.Is6 = a.Is6
				i.Vrf = dom.Name
				items = append(items, &i)
			}
		}
	}
	return items
}

func (a *AddrList) XPath() string {
	if a.Is6 {
		return "System/ipv6-items/inst-items/dom-items"
	}
	return "System/ipv4-items/inst-items/dom-items"
}

type AddrDom struct {
	Name    string `json:"name"`
	IfItems struct {
		IfList gnmiext.List[string, *AddrItem] `json:"If-list,omitzero"`
	} `json:"if-items,omitzero"`
}

func (d *AddrDom) Key() string { return d.Name }

// AddrItem represents the IP address configuration for an interface.
// It can hold either IPv4 or IPv6 addresses, determined by the Is6 field.
type AddrItem struct {
	ID         string `json:"id"`
	Unnumbered string `json:"unnumbered,omitempty"`
	AddrItems  struct {
		AddrList gnmiext.List[string, *IntfAddr] `json:"Addr-list,omitzero"`
	} `json:"addr-items,omitzero"`

	// Is6 indicates whether the addresses are IPv6 (true) or IPv4 (false).
	// This field is not serialized to JSON and is only used internally to
	// determine the correct XPath for the address.
	Is6 bool `json:"-"`

	// Vrf is the VRF Domain in which the address is configured.
	// This field is not serialized to JSON and is only used internally to
	// determine the correct XPath for the address.
	Vrf string `json:"-"`
}

func (*AddrItem) IsListItem() {}

func (a *AddrItem) Key() string { return a.ID }

func (a *AddrItem) XPath() string {
	if a.Is6 {
		return "System/ipv6-items/inst-items/dom-items/Dom-list[name=" + a.Vrf + "]/if-items/If-list[id=" + a.ID + "]"
	}
	return "System/ipv4-items/inst-items/dom-items/Dom-list[name=" + a.Vrf + "]/if-items/If-list[id=" + a.ID + "]"
}

type IntfAddr struct {
	Addr string       `json:"addr"`
	Pref int          `json:"pref"`
	Tag  int          `json:"tag"`
	Type IntfAddrType `json:"type"`
}

func (a *IntfAddr) Key() string { return a.Addr }

type IntfAddrType string

const (
	IntfAddrTypePrimary   IntfAddrType = "primary"
	IntfAddrTypeSecondary IntfAddrType = "secondary"
)

// FabricFwdIf that represents an Interface configured as part of the HMM Fabric Forwarding Instance.
type FabricFwdIf struct {
	AdminSt AdminSt `json:"adminSt"`
	ID      string  `json:"id"`
	Mode    FwdMode `json:"mode"`
}

func (*FabricFwdIf) IsListItem() {}

func (f *FabricFwdIf) XPath() string {
	return "System/hmm-items/fwdinst-items/if-items/FwdIf-list[id=" + f.ID + "]"
}

type FwdMode string

const (
	FwdModeStandard       FwdMode = "standard"
	FwdModeAnycastGateway FwdMode = "anycastGW"
	FwdModeProxyGateway   FwdMode = "proxyGW"
)

type FabricFwdAnycastMAC string

func (*FabricFwdAnycastMAC) XPath() string {
	return "System/hmm-items/fwdinst-items/amac"
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
func Exists(ctx context.Context, client gnmiext.Client, names ...string) (bool, error) {
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
		if matches := vlanRe.FindStringSubmatch(name); matches != nil {
			c = &SwitchVirtualInterface{ID: "vlan" + matches[2]}
		}
		if c == nil {
			return false, fmt.Errorf("unsupported interface format %q, expected one of: %s, %s, %s, %s, %s", name, mgmtRe.String(), ethernetRe.String(), loopbackRe.String(), portchannelRe.String(), vlanRe.String())
		}
		conf = append(conf, c)
	}
	err := client.GetConfig(ctx, conf...)
	if errors.Is(err, gnmiext.ErrNil) {
		return false, nil
	}
	return err == nil, err
}

type Ports struct {
	PhysIfList []*Port `json:"PhysIf-list"`
}

func (p Ports) XPath() string {
	return "System/intf-items/phys-items"
}

type Port struct {
	ID        string `json:"id"`
	PhysItems struct {
		FcotItems struct {
			Description string `json:"description"`
		} `json:"fcot-items"`
		PortcapItems struct {
			Speed string   `json:"speed"`
			Type  ASCIIStr `json:"type"`
		} `json:"portcap-items"`
	} `json:"phys-items"`
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

type SVIMedium string

const (
	SVIMediumBroadcast    SVIMedium = "bcast"
	SVIMediumPointToPoint SVIMedium = "p2p"
)

type SwitchportMode string

const (
	SwitchportModeAccess SwitchportMode = "access"
	SwitchportModeTrunk  SwitchportMode = "trunk"
)

type SpanningTreeMode string

const (
	SpanningTreeModeDefault SpanningTreeMode = "default"
	SpanningTreeModeEdge    SpanningTreeMode = "edge"
	SpanningTreeModeNetwork SpanningTreeMode = "network"
	SpanningTreeModeTrunk   SpanningTreeMode = "trunk"
)

func (s SpanningTreeMode) IsValid() bool {
	switch s {
	case SpanningTreeModeDefault, SpanningTreeModeEdge, SpanningTreeModeNetwork, SpanningTreeModeTrunk:
		return true
	default:
		return false
	}
}

type PortChannelMode string

const (
	PortChannelModeActive  PortChannelMode = "active"
	PortChannelModePassive PortChannelMode = "passive"
)

type MultisiteIfTrackingMode string

const (
	MultisiteIfTrackingModeDCI    MultisiteIfTrackingMode = "dci"
	MultisiteIfTrackingModeFabric MultisiteIfTrackingMode = "fabric"
)

func MultisiteIfTrackingModeFrom(t nxv1alpha1.InterconnectTrackingType) MultisiteIfTrackingMode {
	switch t {
	case nxv1alpha1.InterconnectTrackingTypeDCI:
		return MultisiteIfTrackingModeDCI
	case nxv1alpha1.InterconnectTrackingTypeFabric:
		return MultisiteIfTrackingModeFabric
	default:
		return MultisiteIfTrackingModeDCI
	}
}

// UserFlags represents the user configured flags for an interface.
// It supports a combination of the following flags:
// 1 - admin_state
// 2 - admin_layer
// 4 - admin_router_mac
// 8 - admin_dce_mode
// 16 - admin_mtu
type UserFlags uint8

const (
	UserFlagAdminState UserFlags = 1 << iota
	UserFlagAdminLayer
	UserFlagAdminRouterMac
	UserFlagAdminDceMode
	UserFlagAdminMTU
)

var (
	_ fmt.Stringer     = UserFlags(0)
	_ json.Marshaler   = UserFlags(0)
	_ json.Unmarshaler = (*UserFlags)(nil)
)

// UnmarshalJSON implements json.Unmarshaler.
func (f *UserFlags) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var flags UserFlags
	for flag := range strings.SplitSeq(s, ",") {
		switch strings.TrimSpace(flag) {
		case "admin_state":
			flags |= UserFlagAdminState
		case "admin_layer":
			flags |= UserFlagAdminLayer
		case "admin_router_mac":
			flags |= UserFlagAdminRouterMac
		case "admin_dce_mode":
			flags |= UserFlagAdminDceMode
		case "admin_mtu":
			flags |= UserFlagAdminMTU
		case "":
			// ignore empty flag
		default:
			return fmt.Errorf("interface: unknown user flag %q", flag)
		}
	}
	*f = flags
	return nil
}

// MarshalJSON implements json.Marshaler.
func (f UserFlags) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.String())
}

// String implements fmt.Stringer.
func (f UserFlags) String() string {
	var flags []string
	if f&UserFlagAdminState != 0 {
		flags = append(flags, "admin_state")
	}
	if f&UserFlagAdminLayer != 0 {
		flags = append(flags, "admin_layer")
	}
	if f&UserFlagAdminRouterMac != 0 {
		flags = append(flags, "admin_router_mac")
	}
	if f&UserFlagAdminDceMode != 0 {
		flags = append(flags, "admin_dce_mode")
	}
	if f&UserFlagAdminMTU != 0 {
		flags = append(flags, "admin_mtu")
	}
	slices.Sort(flags)
	return strings.Join(flags, ",")
}
