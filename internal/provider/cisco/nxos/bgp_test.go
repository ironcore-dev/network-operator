// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "k8s.io/utils/ptr"

func init() {
	bgpDom := &BGPDom{Name: DefaultVRFName, RtrID: "1.1.1.1", RtrIDAuto: AdminStDisabled}
	bgpDom.AfItems.DomAfList = []*BGPDomAfItem{
		{
			Type:         AddressFamilyL2EVPN,
			RetainRttAll: AdminStEnabled,
		},
		{
			Type:       AddressFamilyIPv6Unicast,
			MaxExtEcmp: 2,
		},
		{
			Type:       AddressFamilyIPv4Unicast,
			MaxExtEcmp: 4,
		},
	}
	Register("bgp_dom", bgpDom)

	bgp := &BGP{AdminSt: AdminStEnabled, Asn: "65000"}
	Register("bgp", bgp)

	bgpPeer := &BGPPeer{
		Addr:    "1.1.1.1",
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		Name:    "EVPN peering with spine",
		SrcIf:   "lo0",
	}
	bgpPeer.AfItems.PeerAfList = []*BGPPeerAfItem{
		{
			Ctrl:       Option[string]{Value: ptr.To(RouteReflectorClient)},
			SendComExt: AdminStEnabled,
			SendComStd: AdminStEnabled,
			Type:       AddressFamilyL2EVPN,
		},
	}
	Register("bgp_peer", bgpPeer)
}
