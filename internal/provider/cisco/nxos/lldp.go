// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var _ gnmiext.Configurable = (*LLDP)(nil)

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
