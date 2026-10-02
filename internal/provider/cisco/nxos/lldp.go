// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

const (
	defaultLLDPHoldTime  uint16 = 120
	defaultLLDPInitDelay uint16 = 2
)

var (
	_ gnmiext.DataElement = (*LLDP)(nil)
	_ gnmiext.Defaultable = (*LLDP)(nil)
	_ gnmiext.DataElement = (*LLDPIfItems)(nil)
	_ gnmiext.DataElement = (*LLDPIfItem)(nil)
	_ gnmiext.Defaultable = (*LLDPOptTLV)(nil)
)

type LLDP struct {
	// HoldTime is the number of seconds that a receiving device should hold the information sent by another device before discarding it.
	HoldTime uint16 `json:"holdTime"`
	// InitDelay is the number of seconds for LLDP to initialize on any interface.
	InitDelay uint16 `json:"initDelayTime"`
	// PCEnable controls whether LLDP is enabled on port-channel interfaces (equivalent to 'lldp port-channel').
	PCEnable AdminSt `json:"pcEnable"`
	// OptTlvSel is the set of optional TLVs the device advertises.
	OptTlvSel LLDPOptTLV `json:"optTlvSel"`
}

func (*LLDP) IsListItem() {}

func (*LLDP) XPath() string {
	return "System/lldp-items/inst-items"
}

// LLDPIfItems is the list container for fetching per-interface LLDP configuration.
type LLDPIfItems struct {
	IfList gnmiext.List[string, *LLDPIfItem] `json:"If-list,omitzero"`
}

func (*LLDPIfItems) XPath() string {
	return "System/lldp-items/inst-items/if-items"
}

func (l *LLDP) Default() {
	l.HoldTime = defaultLLDPHoldTime
	l.InitDelay = defaultLLDPInitDelay
	l.PCEnable = AdminStDisabled
	l.OptTlvSel.Default()
}

// LLDPOptTLV is the global LLDP optional-TLV selector. NX-OS encodes it as a
// bitmask of the optional TLVs the device advertises. It supports a combination
// of the following flags:
//
//	1    - port-desc
//	2    - sys-name
//	4    - sys-desc
//	8    - sys-cap
//	16   - mgmt-addr-v4
//	32   - port-vlan
//	64   - mgmt-addr-v6
//	128  - dcbxp
//	256  - power-mgmt
//	512  - four-wire-pwr-mgmt
//	2048 - max-framesize
//	4096 - vlan-name
//	8192 - link-aggregation
type LLDPOptTLV uint16

const (
	LLDPOptTLVPortDesc LLDPOptTLV = 1 << iota
	LLDPOptTLVSysName
	LLDPOptTLVSysDesc
	LLDPOptTLVSysCap
	LLDPOptTLVMgmtAddrV4
	LLDPOptTLVPortVLAN
	LLDPOptTLVMgmtAddrV6
	LLDPOptTLVDcbxp
	LLDPOptTLVPowerMgmt
	LLDPOptTLVFourWirePwrMgmt
	LLDPOptTLVEgressQueuing
	LLDPOptTLVMaxFramesize
	LLDPOptTLVVLANName
	LLDPOptTLVLinkAggregation
)

func (t *LLDPOptTLV) Default() {
	*t = LLDPOptTLVPortDesc | LLDPOptTLVSysName | LLDPOptTLVSysDesc | LLDPOptTLVSysCap | LLDPOptTLVMgmtAddrV4 | LLDPOptTLVMgmtAddrV6 | LLDPOptTLVPortVLAN | LLDPOptTLVDcbxp | LLDPOptTLVPowerMgmt | LLDPOptTLVFourWirePwrMgmt | LLDPOptTLVMaxFramesize | LLDPOptTLVVLANName | LLDPOptTLVLinkAggregation
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *LLDPOptTLV) UnmarshalText(b []byte) error {
	var tlv LLDPOptTLV
	for name := range strings.SplitSeq(string(b), ",") {
		switch strings.TrimSpace(name) {
		case "port-desc":
			tlv |= LLDPOptTLVPortDesc
		case "sys-name":
			tlv |= LLDPOptTLVSysName
		case "sys-desc":
			tlv |= LLDPOptTLVSysDesc
		case "sys-cap":
			tlv |= LLDPOptTLVSysCap
		case "mgmt-addr-v4":
			tlv |= LLDPOptTLVMgmtAddrV4
		case "port-vlan":
			tlv |= LLDPOptTLVPortVLAN
		case "mgmt-addr-v6":
			tlv |= LLDPOptTLVMgmtAddrV6
		case "dcbxp":
			tlv |= LLDPOptTLVDcbxp
		case "power-mgmt":
			tlv |= LLDPOptTLVPowerMgmt
		case "four-wire-pwr-mgmt":
			tlv |= LLDPOptTLVFourWirePwrMgmt
		case "max-framesize":
			tlv |= LLDPOptTLVMaxFramesize
		case "vlan-name":
			tlv |= LLDPOptTLVVLANName
		case "link-aggregation":
			tlv |= LLDPOptTLVLinkAggregation
		case "":
			// ignore empty token
		default:
			return fmt.Errorf("lldp: unknown optional TLV %q", name)
		}
	}
	*t = tlv
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (t LLDPOptTLV) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// String implements fmt.Stringer.
func (t LLDPOptTLV) String() string {
	var optTlv []string
	if t&LLDPOptTLVPortDesc != 0 {
		optTlv = append(optTlv, "port-desc")
	}
	if t&LLDPOptTLVSysName != 0 {
		optTlv = append(optTlv, "sys-name")
	}
	if t&LLDPOptTLVSysDesc != 0 {
		optTlv = append(optTlv, "sys-desc")
	}
	if t&LLDPOptTLVSysCap != 0 {
		optTlv = append(optTlv, "sys-cap")
	}
	if t&LLDPOptTLVMgmtAddrV4 != 0 {
		optTlv = append(optTlv, "mgmt-addr-v4")
	}
	if t&LLDPOptTLVPortVLAN != 0 {
		optTlv = append(optTlv, "port-vlan")
	}
	if t&LLDPOptTLVMgmtAddrV6 != 0 {
		optTlv = append(optTlv, "mgmt-addr-v6")
	}
	if t&LLDPOptTLVDcbxp != 0 {
		optTlv = append(optTlv, "dcbxp")
	}
	if t&LLDPOptTLVPowerMgmt != 0 {
		optTlv = append(optTlv, "power-mgmt")
	}
	if t&LLDPOptTLVFourWirePwrMgmt != 0 {
		optTlv = append(optTlv, "four-wire-pwr-mgmt")
	}
	if t&LLDPOptTLVMaxFramesize != 0 {
		optTlv = append(optTlv, "max-framesize")
	}
	if t&LLDPOptTLVVLANName != 0 {
		optTlv = append(optTlv, "vlan-name")
	}
	if t&LLDPOptTLVLinkAggregation != 0 {
		optTlv = append(optTlv, "link-aggregation")
	}
	slices.Sort(optTlv)
	return strings.Join(optTlv, ",")
}

type LLDPIfItem struct {
	InterfaceName string  `json:"id"`
	AdminRxSt     AdminSt `json:"adminRxSt"`
	AdminTxSt     AdminSt `json:"adminTxSt"`
}

func (i *LLDPIfItem) Default() {
	i.AdminRxSt = AdminStEnabled
	i.AdminTxSt = AdminStEnabled

	// Disable LLDP for port-channel interfaces by default.
	if strings.HasPrefix(i.InterfaceName, "po") {
		i.AdminRxSt = AdminStDisabled
		i.AdminTxSt = AdminStDisabled
	}
}

func (i *LLDPIfItem) Key() string { return i.InterfaceName }

func (*LLDPIfItem) IsListItem() {}

func (i *LLDPIfItem) XPath() string {
	return "System/lldp-items/inst-items/if-items/If-list[id=" + i.InterfaceName + "]"
}

type LLDPOper struct {
	OperSt OperSt `json:"operSt"`
}

func (*LLDPOper) IsListItem() {}

func (*LLDPOper) XPath() string {
	return "System/fm-items/lldp-items"
}

// LLDPAdjacencyItems represents the LLDP neighbor information for a single interface.
type LLDPAdjacencyItems struct {
	// ID is the identifier of the interface for which the LLDP neighbor information is being retrieved, e.g., "eth1/1".
	ID       string `json:"-"`
	AdjItems struct {
		AdjEpList []struct {
			ChassisIDT uint8  `json:"chassisIdT"`
			ChassisIDV string `json:"chassisIdV"`
			PortIDT    uint8  `json:"portIdT"`
			PortIDV    string `json:"portIdV"`
			PortDesc   string `json:"portDesc,omitempty"`
			SysName    string `json:"sysName,omitempty"`
			SysDesc    string `json:"sysDesc,omitempty"`
			TTL        int32  `json:"ttl"`
		} `json:"AdjEp-list,omitzero"`
	} `json:"adj-items,omitzero"`
}

func (p *LLDPAdjacencyItems) XPath() string {
	return "System/lldp-items/inst-items/if-items/If-list[id=" + p.ID + "]"
}

func (*LLDPAdjacencyItems) IsListItem() {}
