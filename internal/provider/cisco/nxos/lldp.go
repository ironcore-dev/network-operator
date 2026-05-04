// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/transport/gnmiext"

var _ gnmiext.DataElement = (*LLDP)(nil)

type LLDP struct {
	// HoldTime is the number of seconds that a receiving device should hold the information sent by another device before discarding it.
	HoldTime Option[uint16] `json:"holdTime"`
	// InitDelay is the number of seconds for LLDP to initialize on any interface.
	InitDelay Option[uint16] `json:"initDelayTime"`
	// IfItems contains the per-interface LLDP configuration.
	IfItems struct {
		IfList gnmiext.List[string, *LLDPIfItem] `json:"If-list,omitzero"`
	} `json:"if-items,omitzero"`
}

func (*LLDP) XPath() string {
	return "System/lldp-items/inst-items"
}

func (*LLDP) IsListItem() {}

type LLDPIfItem struct {
	InterfaceName string          `json:"id"`
	AdminRxSt     Option[AdminSt] `json:"adminRxSt"`
	AdminTxSt     Option[AdminSt] `json:"adminTxSt"`
}

func (i *LLDPIfItem) Key() string { return i.InterfaceName }

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
