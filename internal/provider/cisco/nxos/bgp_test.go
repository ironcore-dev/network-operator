// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	bgpDom := &BGPDom{Name: DefaultVRFName, RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDom.AfItems.DomAfList.Set(&BGPDomAfItem{
		Type:         AddressFamilyL2EVPN,
		RetainRttAll: AdminStEnabled,
	})
	Register("bgp_dom", bgpDom)

	bgpDomVrf := &BGPDom{Name: "CC-MGMT", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	Register("bgp_dom_vrf", bgpDomVrf)

	bgpDomAdvPip := &BGPDom{Name: DefaultVRFName, RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDomAdvPip.AfItems.DomAfList.Set(&BGPDomAfItem{
		Type:         AddressFamilyL2EVPN,
		AdvPip:       AdminStEnabled,
		RetainRttAll: AdminStEnabled,
	})
	Register("bgp_dom_advpip", bgpDomAdvPip)

	bgp := &BGP{AdminSt: AdminStEnabled, Asn: "65000"}
	Register("bgp", bgp)

	bgpPeer := &BGPPeer{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		Name:    "EVPN peering with spine",
		SrcIf:   "lo0",
	}
	bgpPeer.AfItems.PeerAfList.Set(&BGPPeerAfItem{
		Ctrl:       Option[string]{Value: new(RouteReflectorClient)},
		SendComExt: AdminStEnabled,
		SendComStd: AdminStEnabled,
		Type:       AddressFamilyL2EVPN,
	})
	Register("bgp_peer", bgpPeer)

	bgwPeer := &MultisitePeer{Addr: "1.1.1.1", PeerType: BorderGatewayPeerTypeFabricExternal}
	Register("bgw_peer", bgwPeer)

	bgpPeerRp := &BGPPeer{
		VRFName: "CC-MGMT",
		Addr:    "10.0.0.1",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
	}
	bgpPeerRpAf := &BGPPeerAfItem{
		SendComExt: AdminStDisabled,
		SendComStd: AdminStDisabled,
		Type:       AddressFamilyIPv4Unicast,
	}
	bgpPeerRpAf.RtCtrlPItems.RtCtrlPList.Set(&BGPPeerAfRtCtrlP{Direction: RtCtrlDirectionIn, RtMap: "ROUTE_MAP_IN"})
	bgpPeerRpAf.RtCtrlPItems.RtCtrlPList.Set(&BGPPeerAfRtCtrlP{Direction: RtCtrlDirectionOut, RtMap: "ROUTE_MAP_OUT"})
	bgpPeerRp.AfItems.PeerAfList.Set(bgpPeerRpAf)
	Register("bgp_dom_rp", bgpPeerRp)

	bgpDomRdst := &BGPDom{Name: "CC-CLOUD01", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	rdstItem := &BGPDomAfItem{Type: AddressFamilyIPv4Unicast, ExportGwIP: AdminStDisabled}
	rdstItem.InterLeakPItems.InterLeakPList.Set(NewInterLeakPDirect("ROUTE_MAP"))
	bgpDomRdst.AfItems.DomAfList.Set(rdstItem)
	Register("bgp_dom_rdst", bgpDomRdst)

	bgpDomExp := &BGPDom{Name: "CC-CLOUD01", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDomExp.AfItems.DomAfList.Set(&BGPDomAfItem{
		Type:       AddressFamilyIPv4Unicast,
		ExportGwIP: AdminStEnabled,
	})
	Register("bgp_dom_exp", bgpDomExp)

	bgpPeerLocalAs := &BGPPeer{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65001",
		AsnType: PeerAsnTypeNone,
	}
	bgpPeerLocalAs.LocalAsnItems.AsnPropagate = AsnPropagateNone
	bgpPeerLocalAs.LocalAsnItems.LocalAsn = "65002"
	Register("bgp_peer_local_as", bgpPeerLocalAs)
}
