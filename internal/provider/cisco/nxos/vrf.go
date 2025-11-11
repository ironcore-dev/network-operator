// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

const DefaultVRFName = "default"

var _ gnmiext.Configurable = (*VRF)(nil)

type VRF struct {
	Encap    string         `json:"encap"`
	L3Vni    bool           `json:"l3vni"`
	Name     string         `json:"name"`
	Descr    Option[string] `json:"descr"`
	DomItems *VRFDomItems   `json:"dom-items,omitempty"`
}

func (*VRF) IsListItem() {}

func (v *VRF) XPath() string {
	return "System/inst-items/Inst-list[name=" + v.Name + "]"
}

type VRFDomItems struct {
	DomList []*VRFDom `json:"Dom-list,omitempty"`
}

type VRFDom struct {
	Name    string         `json:"name"`
	Rd      string         `json:"rd,omitempty"`
	AfItems *VRFDomAfItems `json:"af-items,omitempty"`
}

type VRFDomAfItems struct {
	DomAfList []*VRFDomAf `json:"DomAf-list,omitempty"`
}

type VRFDomAf struct {
	Type      AddressFamily      `json:"type"`
	CtrlItems *VRFDomAfCtrlItems `json:"ctrl-items,omitempty"`
}

type VRFDomAfCtrlItems struct {
	AfCtrlList []*VRFDomAfCtrl `json:"AfCtrl-list,omitempty"`
}

type VRFDomAfCtrl struct {
	Type      AddressFamily `json:"type"`
	RttpItems *VRFRttpItems `json:"rttp-items,omitempty"`
}

type VRFRttpItems struct {
	RttPList []*RttEntry `json:"RttP-list,omitempty"`
}

type RttEntry struct {
	Type     RttEntryType `json:"type"`
	EntItems *RttEntItems `json:"ent-items,omitempty"`
}

type RttEntItems struct {
	RttEntryList []*Rtt `json:"RttEntry-list,omitempty"`
}

type Rtt struct {
	Rtt string `json:"rtt"`
}

type RttEntryType string

const (
	RttEntryTypeImport RttEntryType = "import"
	RttEntryTypeExport RttEntryType = "export"
)
