// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	rttExports := &RttEntry{
		Type:     RttEntryTypeExport,
		EntItems: &RttEntItems{},
	}
	rttExports.EntItems.RttEntryList = []*Rtt{
		{Rtt: "route-target:as2-nn2:65148:4101"},
		{Rtt: "route-target:as2-nn2:65148:1101"},
	}

	rttImports := &RttEntry{
		Type:     RttEntryTypeImport,
		EntItems: &RttEntItems{RttEntryList: append([]*Rtt(nil), rttExports.EntItems.RttEntryList...)},
	}

	ipv4Ctrl := &VRFDomAfCtrl{
		Type:      AddressFamilyIPv4Unicast,
		RttpItems: &VRFRttpItems{RttPList: []*RttEntry{rttExports, rttImports}},
	}

	evpnCtrl := &VRFDomAfCtrl{
		Type:      AddressFamilyL2EVPN,
		RttpItems: &VRFRttpItems{RttPList: []*RttEntry{rttExports, rttImports}},
	}

	af := &VRFDomAf{
		Type:      AddressFamilyIPv4Unicast,
		CtrlItems: &VRFDomAfCtrlItems{AfCtrlList: []*VRFDomAfCtrl{evpnCtrl, ipv4Ctrl}},
	}

	dom := &VRFDom{
		Name:    "CC-CLOUD01",
		Rd:      "rd:as4-nn2:4269539332:101",
		AfItems: &VRFDomAfItems{DomAfList: []*VRFDomAf{af}},
	}

	vrf := &VRF{
		Name:     "CC-CLOUD01",
		L3Vni:    true,
		Encap:    "vxlan-101",
		Descr:    NewOption("CC-CLOUD01 VRF"),
		DomItems: &VRFDomItems{DomList: []*VRFDom{dom}},
	}

	Register("vrf", vrf)
}
