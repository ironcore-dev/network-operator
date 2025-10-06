// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

const DefaultVRFName = "default"

var _ gnmiext.Configurable = (*VRF)(nil)

type VRF struct {
	Encap    string `json:"encap"`
	L3Vni    bool   `json:"l3vni"`
	Name     string `json:"name"`
	DomItems struct {
		DomList []*VRFDom `json:"Dom-list,omitzero"`
	} `json:"dom-items,omitzero"`
}

func (v *VRF) IsListItem() {}

func (v *VRF) XPath() string {
	return "System/inst-items/Inst-list[name=" + v.Name + "]"
}

type VRFDom struct {
	Name    string `json:"name"`
	Rd      string `json:"rd"`
	AfItems struct {
		DomAfList []*VRFDomAf `json:"DomAf-list"`
	} `json:"af-items"`
}

type VRFDomAf struct {
	Type      AddressFamily `json:"type"`
	CtrlItems struct {
		AfCtrlList []*VRFDomAfCtrl `json:"AfCtrl-list"`
	} `json:"ctrl-items"`
}

type VRFDomAfCtrl struct {
	Type      AddressFamily `json:"type"`
	RttpItems struct {
		RttPList []*RttEntry `json:"RttP-list"`
	} `json:"rttp-items"`
}

type RttEntry struct {
	Type     RttEntryType `json:"type"`
	EntItems struct {
		RttEntryList []*Rtt `json:"RttEntry-list"`
	} `json:"ent-items"`
}

type Rtt struct {
	Rtt string `json:"rtt"`
}

type RttEntryType string

const (
	RttEntryTypeImport RttEntryType = "import"
	RttEntryTypeExport RttEntryType = "export"
)
