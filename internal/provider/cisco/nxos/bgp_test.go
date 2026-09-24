// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	bgp := &BGP{AdminSt: AdminStEnabled, Asn: "65000"}
	Register("bgp", bgp)

	Register("bgp_dom", &BGPDom{Name: DefaultVRFName, RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled})

	Register("bgp_dom_vrf", &BGPDom{Name: "CC-MGMT", RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled})

	Register("bgp_dom_af", &BGPDomAfItem{
		VRFName:         DefaultVRFName,
		Type:            AddressFamilyIPv4Unicast,
		ExportGwIP:      AdminStDisabled,
		AdvertL2vpnEvpn: AdminStDisabled,
	})

	Register("bgp_dom_af_advpip", &BGPDomAfItem{
		VRFName:      DefaultVRFName,
		Type:         AddressFamilyL2EVPN,
		AdvPip:       AdminStEnabled,
		RetainRttAll: AdminStEnabled,
	})

	Register("bgp_dom_af_exp", &BGPDomAfItem{
		VRFName:         DefaultVRFName,
		Type:            AddressFamilyIPv4Unicast,
		ExportGwIP:      AdminStEnabled,
		AdvertL2vpnEvpn: AdminStDisabled,
	})

	Register("bgp_dom_af_advl2vpnevpn", &BGPDomAfItem{
		VRFName:         DefaultVRFName,
		Type:            AddressFamilyIPv4Unicast,
		ExportGwIP:      AdminStDisabled,
		AdvertL2vpnEvpn: AdminStEnabled,
	})

	rdstItem := &BGPDomAfItem{VRFName: DefaultVRFName, Type: AddressFamilyIPv4Unicast, ExportGwIP: AdminStDisabled, AdvertL2vpnEvpn: AdminStDisabled}
	rdstItem.InterLeakPItems.InterLeakPList.Set(NewInterLeakPDirect("ROUTE_MAP"))
	Register("bgp_dom_af_rdst", rdstItem)

	bgpPeer := &BGPPeer{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		Name:    "EVPN peering with spine",
		SrcIf:   "lo0",
		TTL:     1,
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
		TTL:     1,
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

	bgpPeerLocalAs := &BGPPeer{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65001",
		AsnType: PeerAsnTypeNone,
		TTL:     1,
	}
	bgpPeerLocalAs.LocalAsnItems.AsnPropagate = AsnPropagateNone
	bgpPeerLocalAs.LocalAsnItems.LocalAsn = "65002"
	Register("bgp_peer_local_as", bgpPeerLocalAs)

	Register("bgp_peer_ebgp_multihop", &BGPPeer{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		TTL:     2,
	})
}
