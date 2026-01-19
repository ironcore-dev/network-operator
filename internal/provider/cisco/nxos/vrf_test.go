// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	rtt := new(RttEntry)
	rtt.Type = RttEntryTypeExport
	rtt.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})

	ctrl := new(VRFDomAfCtrl)
	ctrl.Type = AddressFamilyL2EVPN
	ctrl.RttpItems.RttPList.Set(rtt)

	af := new(VRFDomAf)
	af.Type = AddressFamilyIPv4Unicast
	af.CtrlItems.AfCtrlList.Set(ctrl)

	dom := new(VRFDom)
	dom.Name = "CC-CLOUD01"
	dom.Rd = "rd:as4-nn2:4269539332:101"
	dom.AfItems.DomAfList.Set(af)

	vrf := new(VRF)
	vrf.Name = "CC-CLOUD01"
	vrf.L3Vni = true
	vrf.Encap = NewOption("vxlan-101")
	vrf.Descr = NewOption("CC-CLOUD01 VRF")
	vrf.DomItems.DomList.Set(dom)
	Register("vrf", vrf)
}
