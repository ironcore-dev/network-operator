// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	rttExports := &RttEntry{Type: RttEntryTypeExport}
	rttExports.EntItems.RttEntryList = []*Rtt{{Rtt: "route-target:as2-nn2:65148:4101"}, {Rtt: "route-target:as2-nn2:65148:1101"}}

	rttImports := new(RttEntry)
	*rttImports = *rttExports
	rttImports.Type = RttEntryTypeImport

	ipv4 := &VRFDomAfCtrl{Type: AddressFamilyIPv4Unicast}
	ipv4.RttpItems.RttPList = []*RttEntry{rttExports, rttImports}

	evpn := new(VRFDomAfCtrl)
	*evpn = *ipv4
	evpn.Type = AddressFamilyL2EVPN

	af := &VRFDomAf{Type: AddressFamilyIPv4Unicast}
	af.CtrlItems.AfCtrlList = []*VRFDomAfCtrl{evpn, ipv4}

	dom := &VRFDom{Name: "CC-CLOUD01", Rd: "rd:as4-nn2:4269539332:101"}
	dom.AfItems.DomAfList = []*VRFDomAf{af}

	vrf := &VRF{Name: "CC-CLOUD01", L3Vni: true, Encap: "vxlan-101"}
	vrf.DomItems.DomList = []*VRFDom{dom}

	Register("vrf", vrf)
}
