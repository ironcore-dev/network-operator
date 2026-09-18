// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
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

	// Unnumbered peer with a dynamic AS number ("remote-as external"). The device
	// reports asn as an empty string in that case, so it is omitted from the payload.
	bgpPeerIf := &BGPPeerIf{
		VRFName: DefaultVRFName,
		ID:      "eth1/1",
		AdminSt: AdminStEnabled,
		AsnType: PeerAsnTypeExternal,
		Name:    "Unnumbered peering with spine",
	}
	bgpPeerIf.AfItems.PeerAfList.Set(&BGPPeerAfItem{
		SendComExt: AdminStDisabled,
		SendComStd: AdminStDisabled,
		Type:       AddressFamilyIPv4Unicast,
	})
	Register("bgp_peer_if", bgpPeerIf)

	// Unnumbered peer with an explicit AS number ("remote-as 65020").
	bgpPeerIfAsn := &BGPPeerIf{
		VRFName: DefaultVRFName,
		ID:      "eth1/2",
		AdminSt: AdminStEnabled,
		Asn:     "65020",
		AsnType: PeerAsnTypeNone,
	}
	Register("bgp_peer_if_asn", bgpPeerIfAsn)

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
	rdstItem := &BGPDomAfItem{Type: AddressFamilyIPv4Unicast, ExportGwIP: AdminStDisabled, AdvertL2vpnEvpn: AdminStDisabled}
	rdstItem.InterLeakPItems.InterLeakPList.Set(NewInterLeakPDirect("ROUTE_MAP"))
	bgpDomRdst.AfItems.DomAfList.Set(rdstItem)
	Register("bgp_dom_rdst", bgpDomRdst)

	bgpDomExp := &BGPDom{Name: "CC-CLOUD01", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDomExp.AfItems.DomAfList.Set(&BGPDomAfItem{
		Type:            AddressFamilyIPv4Unicast,
		ExportGwIP:      AdminStEnabled,
		AdvertL2vpnEvpn: AdminStDisabled,
	})
	Register("bgp_dom_exp", bgpDomExp)

	bgpDomAdvL2vpnEvpn := &BGPDom{Name: "CC-CLOUD01", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDomAdvL2vpnEvpn.AfItems.DomAfList.Set(&BGPDomAfItem{
		Type:            AddressFamilyIPv4Unicast,
		ExportGwIP:      AdminStDisabled,
		AdvertL2vpnEvpn: AdminStEnabled,
	})
	Register("bgp_dom_advl2vpnevpn", bgpDomAdvL2vpnEvpn)

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
