// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*PIM)(nil)
	_ gnmiext.Configurable = (*PIMDom)(nil)
	_ gnmiext.Configurable = (*StaticRPItems)(nil)
	_ gnmiext.Configurable = (*AnycastPeerItems)(nil)
	_ gnmiext.Configurable = (*PIMIfItems)(nil)
)

type PIM struct {
	AdminSt   AdminSt `json:"adminSt"`
	InstItems struct {
		AdminSt AdminSt `json:"adminSt"`
	} `json:"inst-items"`
}

func (*PIM) XPath() string {
	return "System/pim-items"
}

type PIMDom struct {
	Name    string  `json:"name"`
	AdminSt AdminSt `json:"adminSt"`
}

func (*PIMDom) IsListItem() {}

func (p *PIMDom) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=" + p.Name + "]"
}

type StaticRPItems struct {
	StaticRPList gnmiext.List[string, *StaticRP] `json:"StaticRP-list,omitzero"`
}

func (*StaticRPItems) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items"
}

// StaticRP represents a static Rendezvous Point (RP) configuration in PIM.
type StaticRP struct {
	Addr           string `json:"addr"`
	RpgrplistItems struct {
		RPGrpListList gnmiext.List[string, *StaticRPGrp] `json:"RPGrpList-list,omitzero"`
	} `json:"rpgrplist-items,omitzero"`
}

func (rp *StaticRP) Key() string { return rp.Addr }

func (*StaticRP) IsListItem() {}

func (s *StaticRP) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items/StaticRP-list[addr=" + s.Addr + "]"
}

type StaticRPGrp struct {
	Bidir       bool   `json:"bidir"`
	GrpListName string `json:"grpListName"`
	Override    bool   `json:"override"`
}

func (g *StaticRPGrp) Key() string { return g.GrpListName }

type AnycastPeerItems struct {
	AcastRPPeerList gnmiext.List[AnycastPeerAddr, *AnycastPeerAddr] `json:"AcastRPPeer-list,omitzero"`
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

func (a *AnycastPeerAddr) Key() AnycastPeerAddr { return *a }

func (a *AnycastPeerAddr) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=" + a.Addr + "][rpSetAddr=" + a.RpSetAddr + "]"
}

// PIMIfItems represents the PIM interface configuration.
// It is used to configure PIM on a specific interface.
type PIMIfItems struct {
	IfList gnmiext.List[string, *PIMIf] `json:"If-list,omitzero"`
}

func (*PIMIfItems) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/if-items"
}

type PIMIf struct {
	ID            string `json:"id"`
	PimSparseMode bool   `json:"pimSparseMode"`
}

func (*PIMIf) IsListItem() {}

func (i *PIMIf) Key() string { return i.ID }

func (i *PIMIf) XPath() string {
	return "System/pim-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + i.ID + "]"
}
