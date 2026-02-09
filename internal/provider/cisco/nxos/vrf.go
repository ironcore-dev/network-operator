// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

const (
	DefaultVRFName    = "default"
	ManagementVRFName = "management"
)

var _ gnmiext.Configurable = (*VRF)(nil)

type VRF struct {
	Encap    Option[string] `json:"encap"`
	L3Vni    bool           `json:"l3vni"`
	Name     string         `json:"name"`
	Descr    Option[string] `json:"descr"`
	DomItems VRFDomItems    `json:"dom-items,omitzero"`
}

func (*VRF) IsListItem() {}

func (v *VRF) XPath() string {
	return "System/inst-items/Inst-list[name=" + v.Name + "]"
}

type VRFDomItems struct {
	DomList gnmiext.List[string, *VRFDom] `json:"Dom-list,omitzero"`
}

type VRFDom struct {
	Name    string        `json:"name"`
	Rd      string        `json:"rd,omitempty"`
	AfItems VRFDomAfItems `json:"af-items,omitzero"`
}

func (d *VRFDom) Key() string { return d.Name }

type VRFDomAfItems struct {
	DomAfList gnmiext.List[AddressFamily, *VRFDomAf] `json:"DomAf-list,omitzero"`
}

type VRFDomAf struct {
	Type      AddressFamily     `json:"type"`
	CtrlItems VRFDomAfCtrlItems `json:"ctrl-items,omitzero"`
}

func (af *VRFDomAf) Key() AddressFamily { return af.Type }

type VRFDomAfCtrlItems struct {
	AfCtrlList gnmiext.List[AddressFamily, *VRFDomAfCtrl] `json:"AfCtrl-list,omitzero"`
}

func (c *VRFDomAfCtrl) Key() AddressFamily { return c.Type }

type VRFDomAfCtrl struct {
	Type      AddressFamily `json:"type"`
	RttpItems VRFRttpItems  `json:"rttp-items,omitzero"`
}

type VRFRttpItems struct {
	RttPList gnmiext.List[RttEntryType, *RttEntry] `json:"RttP-list,omitzero"`
}

type RttEntry struct {
	Type     RttEntryType `json:"type"`
	EntItems RttEntItems  `json:"ent-items,omitzero"`
}

func (r *RttEntry) Key() RttEntryType { return r.Type }

type RttEntItems struct {
	RttEntryList gnmiext.List[string, *Rtt] `json:"RttEntry-list,omitzero"`
}

type Rtt struct {
	Rtt string `json:"rtt"`
}

func (r *Rtt) Key() string { return r.Rtt }

type RttEntryType string

const (
	RttEntryTypeImport RttEntryType = "import"
	RttEntryTypeExport RttEntryType = "export"
)
