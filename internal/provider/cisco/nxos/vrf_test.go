// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	// Note: These route targets will be sorted alphabetically in the output
	rttExportEvpn := new(RttEntry)
	rttExportEvpn.Type = RttEntryTypeExport
	rttExportEvpn.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})
	rttExportEvpn.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:1101"})

	rttImportEvpn := new(RttEntry)
	rttImportEvpn.Type = RttEntryTypeImport
	rttImportEvpn.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})
	rttImportEvpn.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:1101"})

	ctrlEvpn := new(VRFDomAfCtrl)
	ctrlEvpn.Type = AddressFamilyL2EVPN
	ctrlEvpn.RttpItems.RttPList.Set(rttExportEvpn)
	ctrlEvpn.RttpItems.RttPList.Set(rttImportEvpn)

	rttExportIpv4 := new(RttEntry)
	rttExportIpv4.Type = RttEntryTypeExport
	rttExportIpv4.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})
	rttExportIpv4.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:1101"})

	rttImportIpv4 := new(RttEntry)
	rttImportIpv4.Type = RttEntryTypeImport
	rttImportIpv4.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:4101"})
	rttImportIpv4.EntItems.RttEntryList.Set(&Rtt{Rtt: "route-target:as2-nn2:65148:1101"})

	ctrlIpv4 := new(VRFDomAfCtrl)
	ctrlIpv4.Type = AddressFamilyIPv4Unicast
	ctrlIpv4.RttpItems.RttPList.Set(rttExportIpv4)
	ctrlIpv4.RttpItems.RttPList.Set(rttImportIpv4)

	af := new(VRFDomAf)
	af.Type = AddressFamilyIPv4Unicast
	af.CtrlItems.AfCtrlList.Set(ctrlEvpn)
	af.CtrlItems.AfCtrlList.Set(ctrlIpv4)

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
