// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"maps"
	"testing"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

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

	bgpPeer := &BGPPeerAddr{
		VRFName: DefaultVRFName,
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		Name:    "EVPN peering with spine",
		Addr:    "1.1.1.1",
		SrcIf:   "lo0",
		BfdType: BfdTypeNone,
	}
	bgpPeer.AfItems.PeerAfList.Set(&BGPPeerAfItem{
		Ctrl:       Option[string]{Value: NewOption(string(RouteReflectorClient)).Value},
		SendComExt: AdminStEnabled,
		SendComStd: AdminStEnabled,
		Type:       AddressFamilyL2EVPN,
	})
	Register("bgp_peer", bgpPeer)

	// Unnumbered peer with a dynamic AS number ("remote-as external"). The device
	// reports asn as an empty string in that case, so it is omitted from the payload.
	bgpPeerIf := &BGPPeerIf{
		VRFName: DefaultVRFName,
		AdminSt: AdminStEnabled,
		AsnType: PeerAsnTypeExternal,
		Name:    "Unnumbered peering with spine",
		ID:      "eth1/1",
		BfdType: BfdTypeNone,
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
		AdminSt: AdminStEnabled,
		Asn:     "65020",
		AsnType: PeerAsnTypeNone,
		ID:      "eth1/2",
		BfdType: BfdTypeNone,
	}
	Register("bgp_peer_if_asn", bgpPeerIfAsn)

	bgwPeer := &MultisitePeer{Addr: "1.1.1.1", PeerType: BorderGatewayPeerTypeFabricExternal}
	Register("bgw_peer", bgwPeer)

	bgpPeerRp := &BGPPeerAddr{
		VRFName: "CC-MGMT",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		Addr:    "10.0.0.1",
		BfdType: BfdTypeNone,
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

	bgpPeerLocalAs := &BGPPeerAddr{
		VRFName: DefaultVRFName,
		AdminSt: AdminStEnabled,
		Asn:     "65001",
		AsnType: PeerAsnTypeNone,
		Addr:    "10.0.0.2",
		BfdType: BfdTypeNone,
	}
	bgpPeerLocalAs.LocalAsnItems.AsnPropagate = AsnPropagateNone
	bgpPeerLocalAs.LocalAsnItems.LocalAsn = "65002"
	Register("bgp_peer_local_as", bgpPeerLocalAs)

	Register("bgp_peer_ebgp_multihop", &BGPPeerAddr{
		VRFName: DefaultVRFName,
		Addr:    "1.1.1.1",
		AdminSt: AdminStEnabled,
		Asn:     "65000",
		AsnType: PeerAsnTypeNone,
		TTL:     NewOption[int32](2),
		BfdType: BfdTypeNone,
	})
}

func TestValidateRoutingPolicies(t *testing.T) {
	self := func() *v1alpha1.RoutingPolicy {
		return newRoutingPolicy(newAcceptStatement(10, &v1alpha1.SetNextHopAction{Type: v1alpha1.NextHopSelf}))
	}
	withoutNextHop := func() *v1alpha1.RoutingPolicy {
		return newRoutingPolicy(newAcceptStatement(10, nil))
	}
	addr := v1alpha1.MustParseAddr("192.0.2.1")
	address := func() *v1alpha1.RoutingPolicy {
		return newRoutingPolicy(newAcceptStatement(10, &v1alpha1.SetNextHopAction{
			Type:   v1alpha1.NextHopAddress,
			IPAddr: &addr,
		}))
	}

	tests := []struct {
		name     string
		inbound  map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy
		outbound map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy
		want     map[v1alpha1.BGPAddressFamilyType]bool
		wantErr  bool
	}{
		{
			name:     "no next-hop action",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyIpv4Unicast: withoutNextHop()},
			want:     map[v1alpha1.BGPAddressFamilyType]bool{},
		},
		{
			name:     "outbound self",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyIpv4Unicast: self()},
			want:     map[v1alpha1.BGPAddressFamilyType]bool{v1alpha1.BGPAddressFamilyIpv4Unicast: true},
		},
		{
			name: "self with reject fallback",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{
				v1alpha1.BGPAddressFamilyIpv4Unicast: newRoutingPolicy(
					newAcceptStatement(10, &v1alpha1.SetNextHopAction{Type: v1alpha1.NextHopSelf}),
					&v1alpha1.PolicyStatement{Sequence: 20, Actions: v1alpha1.PolicyActions{RouteDisposition: v1alpha1.RejectRoute}},
				),
			},
			want: map[v1alpha1.BGPAddressFamilyType]bool{v1alpha1.BGPAddressFamilyIpv4Unicast: true},
		},
		{
			name: "address families are independent",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{
				v1alpha1.BGPAddressFamilyIpv4Unicast: self(),
				v1alpha1.BGPAddressFamilyIpv6Unicast: withoutNextHop(),
			},
			want: map[v1alpha1.BGPAddressFamilyType]bool{v1alpha1.BGPAddressFamilyIpv4Unicast: true},
		},
		{
			name: "mixed accepted routes",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{
				v1alpha1.BGPAddressFamilyIpv4Unicast: newRoutingPolicy(
					newAcceptStatement(10, &v1alpha1.SetNextHopAction{Type: v1alpha1.NextHopSelf}),
					newAcceptStatement(20, nil),
				),
			},
			wantErr: true,
		},
		{
			name:    "inbound self",
			inbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyIpv4Unicast: self()},
			wantErr: true,
		},
		{
			name:    "inbound address",
			inbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyIpv4Unicast: address()},
			wantErr: true,
		},
		{
			name:     "outbound address",
			outbound: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyIpv4Unicast: address()},
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateRoutingPolicies(test.inbound, test.outbound)
			if test.wantErr {
				if err == nil {
					t.Fatal("validateRoutingPolicies() error = nil")
				}
				status, ok := apistatus.FromError(err)
				if !ok || status.Code != apistatus.CodeUnsupportedField {
					t.Fatalf("validateRoutingPolicies() error = %v, want UnsupportedField", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRoutingPolicies() error = %v", err)
			}
			if !maps.Equal(got, test.want) {
				t.Errorf("validateRoutingPolicies() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBGPPeerAfItemsPreservesControlsAndPolicies(t *testing.T) {
	inbound := &v1alpha1.RoutingPolicy{Spec: v1alpha1.RoutingPolicySpec{Name: "ROUTE_MAP_IN"}}
	outbound := &v1alpha1.RoutingPolicy{Spec: v1alpha1.RoutingPolicySpec{Name: "ROUTE_MAP_OUT"}}
	req := &provider.EnsureBGPPeerRequest{
		BGPPeer: &v1alpha1.BGPPeer{Spec: v1alpha1.BGPPeerSpec{
			AddressFamilies: &v1alpha1.BGPPeerAddressFamilies{
				L2vpnEvpn: &v1alpha1.BGPPeerAddressFamily{Enabled: true, RouteReflectorClient: true},
			},
		}},
		InboundRoutingPolicies:  map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyL2vpnEvpn: inbound},
		OutboundRoutingPolicies: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{v1alpha1.BGPAddressFamilyL2vpnEvpn: outbound},
	}

	items := bgpPeerAfItems(req, map[v1alpha1.BGPAddressFamilyType]bool{v1alpha1.BGPAddressFamilyL2vpnEvpn: true})
	item, ok := items.Get(AddressFamilyL2EVPN)
	if !ok {
		t.Fatal("bgpPeerAfItems() did not return l2vpn-evpn")
	}
	if item.Ctrl.Value == nil || *item.Ctrl.Value != "rr-client,nh-self" {
		t.Fatalf("bgpPeerAfItems() ctrl = %v, want rr-client,nh-self", item.Ctrl.Value)
	}
	in, ok := item.RtCtrlPItems.RtCtrlPList.Get(RtCtrlDirectionIn)
	if !ok || in.RtMap != "ROUTE_MAP_IN" {
		t.Fatalf("bgpPeerAfItems() inbound policy = %v, want ROUTE_MAP_IN", in)
	}
	out, ok := item.RtCtrlPItems.RtCtrlPList.Get(RtCtrlDirectionOut)
	if !ok || out.RtMap != "ROUTE_MAP_OUT" {
		t.Fatalf("bgpPeerAfItems() outbound policy = %v, want ROUTE_MAP_OUT", out)
	}

	items = bgpPeerAfItems(req, nil)
	item, ok = items.Get(AddressFamilyL2EVPN)
	if !ok {
		t.Fatal("bgpPeerAfItems() did not return l2vpn-evpn after removing next-hop-self")
	}
	if item.Ctrl.Value == nil || *item.Ctrl.Value != "rr-client" {
		t.Fatalf("bgpPeerAfItems() ctrl after removing next-hop-self = %v, want rr-client", item.Ctrl.Value)
	}
	out, ok = item.RtCtrlPItems.RtCtrlPList.Get(RtCtrlDirectionOut)
	if !ok || out.RtMap != "ROUTE_MAP_OUT" {
		t.Fatalf("bgpPeerAfItems() outbound policy after removing next-hop-self = %v, want ROUTE_MAP_OUT", out)
	}
}

func newRoutingPolicy(statements ...*v1alpha1.PolicyStatement) *v1alpha1.RoutingPolicy {
	policy := &v1alpha1.RoutingPolicy{}
	for _, statement := range statements {
		policy.Spec.Statements = append(policy.Spec.Statements, *statement)
	}
	return policy
}

func newAcceptStatement(sequence int32, nextHop *v1alpha1.SetNextHopAction) *v1alpha1.PolicyStatement {
	statement := &v1alpha1.PolicyStatement{
		Sequence: sequence,
		Actions:  v1alpha1.PolicyActions{RouteDisposition: v1alpha1.AcceptRoute},
	}
	if nextHop != nil {
		statement.Actions.BgpActions = &v1alpha1.BgpActions{SetNextHop: nextHop}
	}
	return statement
}
