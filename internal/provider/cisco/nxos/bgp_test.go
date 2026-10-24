// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
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
		Ctrl:       RouteReflectorClient,
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

func TestEnsureBGPRedistributionNextHop(t *testing.T) {
	addr := v1alpha1.MustParseAddr("192.0.2.1")
	tests := []struct {
		name    string
		af      v1alpha1.BGPAddressFamilyType
		nextHop *v1alpha1.SetNextHopAction
	}{
		{name: "IPv4 address", af: v1alpha1.BGPAddressFamilyIpv4Unicast, nextHop: &v1alpha1.SetNextHopAction{Type: v1alpha1.NextHopAddress, IPAddr: &addr}},
		{name: "IPv6 self", af: v1alpha1.BGPAddressFamilyIpv6Unicast, nextHop: &v1alpha1.SetNextHopAction{Type: v1alpha1.NextHopSelf}},
		{name: "no next-hop action", af: v1alpha1.BGPAddressFamilyIpv4Unicast},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deviceErr := errors.New("device error")
			client := &gnmiext.ClientMock{
				GetConfigFunc: func(context.Context, ...gnmiext.DataElement) error { return deviceErr },
			}
			p := &Provider{client: client}
			err := p.EnsureBGP(t.Context(), &provider.EnsureBGPRequest{
				RedistributeDirectRoutePolicies: map[v1alpha1.BGPAddressFamilyType]*v1alpha1.RoutingPolicy{
					test.af: newRoutingPolicy(newAcceptStatement(10, nil), newAcceptStatement(20, test.nextHop)),
				},
			})
			if test.nextHop == nil {
				if !errors.Is(err, deviceErr) {
					t.Fatalf("EnsureBGP() error = %v, want device error", err)
				}
				return
			}
			status, ok := apistatus.FromError(err)
			if !ok || status.Code != apistatus.CodeUnsupportedField {
				t.Fatalf("EnsureBGP() error = %v, want UnsupportedField", err)
			}
			wantField := "spec.addressFamilies." + string(test.af) + ".redistributeDirectRoutes.routingPolicyRef"
			if len(status.FieldViolations) != 1 || status.FieldViolations[0].Field != wantField {
				t.Errorf("EnsureBGP() violations = %v, want field %s", status.FieldViolations, wantField)
			}
			if len(client.GetConfigCalls()) != 0 {
				t.Error("EnsureBGP() contacted the device before rejecting an omitted next-hop action")
			}
		})
	}
}

func TestPeerAfControl(t *testing.T) {
	tests := []struct {
		control string
		want    PeerAfControl
		round   string
	}{
		{control: "rr-client", want: RouteReflectorClient, round: "rr-client"},
		{control: "nh-self", want: NextHopSelf, round: "nh-self"},
		{control: "dis-peer-as-check", want: DisablePeerASCheck, round: "dis-peer-as-check"},
		{control: "allow-self-as", want: AllowSelfAS, round: "allow-self-as"},
		{control: "default-originate", want: DefaultOriginate, round: "default-originate"},
		{control: "advertisement-interval", want: AdvertisementInterval, round: "advertisement-interval"},
		{control: "suppress-inactive", want: SuppressInactive, round: "suppress-inactive"},
		{control: "nh-self-all", want: NextHopSelfAll, round: "nh-self-all"},
		{control: "rr-client,nh-self", want: RouteReflectorClient | NextHopSelf, round: "nh-self,rr-client"},
		{control: "nh-self,rr-client", want: RouteReflectorClient | NextHopSelf, round: "nh-self,rr-client"},
		{control: "", want: 0, round: ""},
	}
	for _, test := range tests {
		var got PeerAfControl
		if err := got.UnmarshalText([]byte(test.control)); err != nil {
			t.Errorf("UnmarshalText(%q) returned %v", test.control, err)
			continue
		}
		if got != test.want {
			t.Errorf("UnmarshalText(%q) = %d, want %d", test.control, got, test.want)
		}
		if s := got.String(); s != test.round {
			t.Errorf("UnmarshalText(%q).String() = %q, want %q", test.control, s, test.round)
		}
	}
	var control PeerAfControl
	if err := control.UnmarshalText([]byte("rr-client,bogus")); err == nil {
		t.Error("UnmarshalText did not reject an unknown control")
	}
}

func TestPeerAfControlJSONOrderIndependent(t *testing.T) {
	want := RouteReflectorClient | NextHopSelf | DisablePeerASCheck
	for _, control := range []string{"rr-client,dis-peer-as-check,nh-self", "nh-self,rr-client,dis-peer-as-check"} {
		var item BGPPeerAfItem
		if err := json.Unmarshal([]byte(`{"ctrl":"`+control+`"}`), &item); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if item.Ctrl != want {
			t.Errorf("json.Unmarshal() control = %d, want %d", item.Ctrl, want)
		}
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(b) != `"dis-peer-as-check,nh-self,rr-client"` {
		t.Errorf("json.Marshal() = %s, want %q", b, "dis-peer-as-check,nh-self,rr-client")
	}
	var unset BGPPeerAfItem
	if err := json.Unmarshal([]byte(`{"ctrl":"DME_UNSET_PROPERTY_MARKER"}`), &unset); err != nil {
		t.Fatalf("json.Unmarshal() unset marker error = %v", err)
	}
	if unset.Ctrl != 0 {
		t.Errorf("json.Unmarshal() unset marker control = %d, want 0", unset.Ctrl)
	}
}

func TestBGPPeerControlDriftReconciliation(t *testing.T) {
	state := []byte(`{"addr":"192.0.2.1","ttl":2,"af-items":{"PeerAf-list":[{"type":"ipv4-ucast","ctrl":"dis-peer-as-check,nh-self"}]}}`)
	sets := 0
	conn := &bgpControlClientConn{invoke: func(_ context.Context, _ string, args, reply any, _ ...grpc.CallOption) error {
		switch req := args.(type) {
		case *gpb.CapabilityRequest:
			reply.(*gpb.CapabilityResponse).SupportedEncodings = []gpb.Encoding{gpb.Encoding_JSON}
		case *gpb.GetRequest:
			reply.(*gpb.GetResponse).Notification = []*gpb.Notification{{Update: []*gpb.Update{{
				Path: req.GetPath()[0], Val: &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: state}},
			}}}}
		case *gpb.SetRequest:
			sets++
			state = req.GetUpdate()[0].GetVal().GetJsonVal()
			// NX-OS reads unset properties back as null.
			state = bytes.ReplaceAll(state, []byte(`"DME_UNSET_PROPERTY_MARKER"`), []byte(`null`))
		default:
			t.Fatalf("unexpected gNMI request %T", req)
		}
		return nil
	}}
	client, err := gnmiext.New(t.Context(), conn)
	if err != nil {
		t.Fatal(err)
	}
	peer := &BGPPeerAddr{VRFName: DefaultVRFName, Addr: "192.0.2.1", TTL: NewOption[int32](2)}
	peer.AfItems.PeerAfList.Set(&BGPPeerAfItem{Type: AddressFamilyIPv4Unicast, Ctrl: NextHopSelf})
	for range 2 {
		if err := client.Patch(t.Context(), peer); err != nil {
			t.Fatalf("Patch() error = %v", err)
		}
	}
	if sets != 1 {
		t.Errorf("got %d Set requests, want one drift correction and no subsequent write", sets)
	}
}

type bgpControlClientConn struct {
	grpc.ClientConnInterface
	invoke func(context.Context, string, any, any, ...grpc.CallOption) error
}

func (c *bgpControlClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return c.invoke(ctx, method, args, reply, opts...)
}

func TestPeerAfControlJSONNullIsUpToDate(t *testing.T) {
	want := BGPPeerAfItem{}
	var got BGPPeerAfItem
	if err := json.Unmarshal([]byte(`{"ctrl":null}`), &got); err != nil {
		t.Fatalf("json.Unmarshal() null control error = %v", err)
	}
	if got.Ctrl != 0 {
		t.Fatalf("json.Unmarshal() null control = %d, want 0", got.Ctrl)
	}
	if !reflect.DeepEqual(&want, &got) {
		t.Errorf("null control read-back differs from desired unset control: got %#v, want %#v", got, want)
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
	if item.Ctrl != RouteReflectorClient|NextHopSelf {
		t.Fatalf("bgpPeerAfItems() ctrl = %v, want rr-client and nh-self", item.Ctrl)
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
	if item.Ctrl != RouteReflectorClient {
		t.Fatalf("bgpPeerAfItems() ctrl after removing next-hop-self = %v, want rr-client", item.Ctrl)
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
