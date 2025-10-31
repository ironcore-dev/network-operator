// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*StaticRPItems)(nil)
	_ gnmiext.Configurable = (*AnycastPeerItems)(nil)
	_ gnmiext.Configurable = (*PIMIfItems)(nil)
)

type StaticRPItems struct {
	StaticRPList []*StaticRP `json:"StaticRP-list"`
}

func (*StaticRPItems) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items"
}

// StaticRP represents a static Rendezvous Point (RP) configuration in PIM.
type StaticRP struct {
	Addr           string `json:"addr"`
	RpgrplistItems struct {
		RPGrpListList []*StaticRPGrp `json:"RPGrpList-list,omitempty"`
	} `json:"rpgrplist-items,omitzero"`
}

func (*StaticRP) IsListItem() {}

func (s *StaticRP) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items/StaticRP-list[addr=" + s.Addr + "]"
}

type StaticRPGrp struct {
	Bidir       bool   `json:"bidir"`
	GrpListName string `json:"grpListName"`
	Override    bool   `json:"override"`
}

type AnycastPeerItems struct {
	AcastRPPeerList []*AnycastPeerAddr `json:"AcastRPPeer-list,omitempty"`
}

func (*AnycastPeerItems) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/acastrpfunc-items/peer-items"
}

// AnycastPeerAddr represents an anycast RP peer address configuration used for redundancy in PIM.
type AnycastPeerAddr struct {
	Addr      string `json:"addr"`
	RpSetAddr string `json:"rpSetAddr"`
}

func (*AnycastPeerAddr) IsListItem() {}

func (a *AnycastPeerAddr) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=" + a.Addr + "][rpSetAddr=" + a.RpSetAddr + "]"
}

// PIMIfItems represents the PIM interface configuration.
// It is used to configure PIM on a specific interface.
type PIMIfItems struct {
	IfList []*PIMIf `json:"If-list,omitempty"`
}

func (*PIMIfItems) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/if-items"
}

type PIMIf struct {
	ID            string `json:"id"`
	PimSparseMode bool   `json:"pimSparseMode"`
}

func (*PIMIf) IsListItem() {}

func (i *PIMIf) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + i.ID + "]"
}
