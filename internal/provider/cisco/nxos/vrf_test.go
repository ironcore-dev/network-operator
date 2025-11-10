// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

func init() {
	rtt := new(RttEntry)
	rtt.Type = RttEntryTypeExport
	rtt.EntItems.RttEntryList = make(gnmiext.List[string, *Rtt])
	rtt.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})

	ctrl := new(VRFDomAfCtrl)
	ctrl.Type = AddressFamilyL2EVPN
	ctrl.RttpItems.RttPList = make(gnmiext.List[RttEntryType, *RttEntry])
	ctrl.RttpItems.RttPList.Set(rtt)

	af := new(VRFDomAf)
	af.Type = AddressFamilyIPv4Unicast
	af.CtrlItems.AfCtrlList = make(gnmiext.List[AddressFamily, *VRFDomAfCtrl])
	af.CtrlItems.AfCtrlList.Set(ctrl)

	dom := new(VRFDom)
	dom.Name = "CC-CLOUD01"
	dom.Rd = "rd:as4-nn2:4269539332:101"
	dom.AfItems.DomAfList = make(gnmiext.List[AddressFamily, *VRFDomAf])
	dom.AfItems.DomAfList.Set(af)

	vrf := new(VRF)
	vrf.Name = "CC-CLOUD01"
	vrf.L3Vni = true
	vrf.Encap = "vxlan-101"
	vrf.Descr = NewOption("CC-CLOUD01 VRF")
	vrf.DomItems.DomList = make(gnmiext.List[string, *VRFDom])
	vrf.DomItems.DomList.Set(dom)
	Register("vrf", vrf)
}
