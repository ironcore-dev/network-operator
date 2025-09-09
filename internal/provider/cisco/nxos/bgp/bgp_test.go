// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package bgp

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestNewBGP(t *testing.T) {
	tests := []struct {
		name     string
		routerID string
		asn      uint32
		opts     []BGPOption
		wantErr  bool
	}{
		{
			name:     "valid BGP instance",
			routerID: "192.168.1.1",
			asn:      65001,
			wantErr:  false,
		},
		{
			name:     "empty router ID",
			routerID: "",
			asn:      65001,
			wantErr:  true,
		},
		{
			name:     "invalid router ID format",
			routerID: "invalid-ip",
			asn:      65001,
			wantErr:  true,
		},
		{
			name:     "IPv6 router ID",
			routerID: "2001:db8::1",
			asn:      65001,
			wantErr:  true,
		},
		{
			name:     "zero ASN",
			routerID: "192.168.1.1",
			asn:      0,
			wantErr:  true,
		},
		{
			name:     "ASN too large",
			routerID: "192.168.1.1",
			asn:      65536,
			wantErr:  true,
		},
		{
			name:     "with address family",
			routerID: "192.168.1.1",
			asn:      65001,
			opts:     []BGPOption{WithAddressFamily(&IPv4Unicast{})},
			wantErr:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bgp, err := NewBGP(test.routerID, test.asn, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewBGP() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewBGP() unexpected error = %v", err)
				return
			}
			if bgp == nil {
				t.Errorf("NewBGP() returned nil BGP instance")
				return
			}
			if bgp.AsNumber != "65001" {
				t.Errorf("NewBGP() AsNumber = %v, want %v", bgp.AsNumber, "65001")
			}
			expectedIP := netip.MustParseAddr(test.routerID)
			if bgp.RouterID != expectedIP {
				t.Errorf("NewBGP() RouterID = %v, want %v", bgp.RouterID, expectedIP)
			}
		})
	}
}

func TestWithAddressFamily(t *testing.T) {
	tests := []struct {
		name    string
		af      AddressFamily
		wantErr bool
	}{
		{
			name:    "nil address family",
			af:      nil,
			wantErr: true,
		},
		{
			name:    "valid L2EVPN",
			af:      &L2EVPN{},
			wantErr: false,
		},
		{
			name:    "valid IPv4Unicast",
			af:      &IPv4Unicast{},
			wantErr: false,
		},
		{
			name:    "valid IPv6Unicast",
			af:      &IPv6Unicast{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bgp := &BGP{}
			opt := WithAddressFamily(test.af)
			err := opt(bgp)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithAddressFamily() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithAddressFamily() unexpected error = %v", err)
				return
			}
			if len(bgp.AddressFamilies) != 1 {
				t.Errorf("WithAddressFamily() expected 1 address family, got %d", len(bgp.AddressFamilies))
			}
		})
	}
}

func TestWithAddressFamily_Duplicate(t *testing.T) {
	bgp := &BGP{}
	af1 := &IPv4Unicast{}
	af2 := &IPv4Unicast{}

	// Add first address family
	opt1 := WithAddressFamily(af1)
	if err := opt1(bgp); err != nil {
		t.Fatalf("WithAddressFamily() unexpected error = %v", err)
	}

	// Try to add duplicate address family
	opt2 := WithAddressFamily(af2)
	err := opt2(bgp)
	if err == nil {
		t.Errorf("WithAddressFamily() expected error for duplicate address family, got nil")
		return
	}
	expectedErr := "bgp: address family *bgp.IPv4Unicast already exists"
	if err.Error() != expectedErr {
		t.Errorf("WithAddressFamily() error = %v, want %v", err, expectedErr)
	}
}

func TestBGP_ToYGOT(t *testing.T) {
	bgp, err := NewBGP("192.168.1.1", 65001, WithAddressFamily(&IPv4Unicast{}))
	if err != nil {
		t.Fatalf("NewBGP() unexpected error = %v", err)
	}

	got, err := bgp.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("BGP.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("BGP.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check BGP feature enablement
	update1, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected first update to be ReplacingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/bgp-items" {
		t.Errorf("BGP.ToYGOT() expected XPath 'System/fm-items/bgp-items', got %v", update1.XPath)
	}

	bgpItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems")
		return
	}
	if bgpItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("BGP.ToYGOT() expected BGP AdminSt to be enabled")
	}

	// Check BGP instance configuration
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}
	if update2.XPath != "System/bgp-items/inst-items" {
		t.Errorf("BGP.ToYGOT() expected XPath 'System/bgp-items/inst-items', got %v", update2.XPath)
	}

	inst, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems")
		return
	}

	if inst.AdminSt != nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled {
		t.Errorf("BGP.ToYGOT() expected instance AdminSt to be enabled")
	}
	if *inst.Asn != "65001" {
		t.Errorf("BGP.ToYGOT() expected ASN '65001', got %v", *inst.Asn)
	}

	domList := inst.GetDomItems().GetDomList("default")
	if domList == nil {
		t.Errorf("BGP.ToYGOT() expected default domain to be present")
		return
	}
	if *domList.RtrId != "192.168.1.1" {
		t.Errorf("BGP.ToYGOT() expected router ID '192.168.1.1', got %v", *domList.RtrId)
	}
	if domList.RtrIdAuto != nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled {
		t.Errorf("BGP.ToYGOT() expected router ID auto to be disabled")
	}
}

func TestBGP_ToYGOT_WithEVPN(t *testing.T) {
	bgp, err := NewBGP("192.168.1.1", 65001, WithAddressFamily(&L2EVPN{}))
	if err != nil {
		t.Fatalf("NewBGP() unexpected error = %v", err)
	}

	got, err := bgp.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("BGP.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 3 {
		t.Errorf("BGP.ToYGOT() expected 3 updates with EVPN, got %d", len(got))
		return
	}

	// Check EVPN feature enablement (should be first update)
	update1, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected first update to be ReplacingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/evpn-items" {
		t.Errorf("BGP.ToYGOT() expected XPath 'System/fm-items/evpn-items', got %v", update1.XPath)
	}

	evpnItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_EvpnItems)
	if !ok {
		t.Errorf("BGP.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_EvpnItems")
		return
	}
	if evpnItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("BGP.ToYGOT() expected EVPN AdminSt to be enabled")
	}
}

func TestBGP_Reset(t *testing.T) {
	bgp, err := NewBGP("192.168.1.1", 65001)
	if err != nil {
		t.Fatalf("NewBGP() unexpected error = %v", err)
	}

	got, err := bgp.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("BGP.Reset() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("BGP.Reset() expected 2 updates, got %d", len(got))
		return
	}

	// Check BGP feature disablement
	update1, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("BGP.Reset() expected first update to be EditingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/bgp-items" {
		t.Errorf("BGP.Reset() expected XPath 'System/fm-items/bgp-items', got %v", update1.XPath)
	}

	bgpItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems)
	if !ok {
		t.Errorf("BGP.Reset() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems")
		return
	}
	if bgpItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled {
		t.Errorf("BGP.Reset() expected BGP AdminSt to be disabled")
	}

	// Check BGP deletion
	update2, ok := got[1].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("BGP.Reset() expected second update to be DeletingUpdate")
		return
	}
	if update2.XPath != "System/bgp-items" {
		t.Errorf("BGP.Reset() expected XPath 'System/bgp-items', got %v", update2.XPath)
	}
}

func TestAddressFamily_toYGOT(t *testing.T) {
	tests := []struct {
		name string
		af   AddressFamily
		want *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList
	}{
		{
			name: "default L2EVPN",
			af:   &L2EVPN{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type: nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
			},
		},
		{
			name: "L2EVPN with maximum paths",
			af:   &L2EVPN{MaximumPaths: 4},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				MaxExtEcmp: ygot.Uint8(4),
			},
		},
		{
			name: "L2EVPN with retain route target all",
			af:   &L2EVPN{RetainRouteTarget: "all"},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type:           nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				RetainRttAll:   nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				RetainRttRtMap: ygot.String("DME_UNSET_PROPERTY_MARKER"),
			},
		},
		{
			name: "L2EVPN with retain route target map",
			af:   &L2EVPN{RetainRouteTarget: "my-route-map"},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type:           nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				RetainRttAll:   nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				RetainRttRtMap: ygot.String("my-route-map"),
			},
		},
		{
			name: "IPv4Unicast",
			af:   &IPv4Unicast{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type: nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast,
			},
		},
		{
			name: "IPv6Unicast",
			af:   &IPv6Unicast{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{
				Type: nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast,
			},
		},
	}

	for _, tests := range tests {
		t.Run(tests.name, func(t *testing.T) {
			got := tests.af.toYGOT()
			if !reflect.DeepEqual(got, tests.want) {
				t.Errorf("%T.toYGOT() = %+v, want %+v", tests.af, got, tests.want)
			}
		})
	}
}

func TestNewBGPPeer(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		asn     uint32
		srcIf   string
		opts    []BGPPeerOption
		wantErr bool
	}{
		{
			name:    "valid BGP peer",
			addr:    "192.168.1.2",
			asn:     65002,
			srcIf:   "loopback0",
			wantErr: false,
		},
		{
			name:    "empty address",
			addr:    "",
			asn:     65002,
			srcIf:   "loopback0",
			wantErr: true,
		},
		{
			name:    "invalid address format",
			addr:    "invalid-ip",
			asn:     65002,
			srcIf:   "loopback0",
			wantErr: true,
		},
		{
			name:    "zero ASN",
			addr:    "192.168.1.2",
			asn:     0,
			srcIf:   "loopback0",
			wantErr: true,
		},
		{
			name:    "ASN too large",
			addr:    "192.168.1.2",
			asn:     65536,
			srcIf:   "loopback0",
			wantErr: true,
		},
		{
			name:    "empty source interface",
			addr:    "192.168.1.2",
			asn:     65002,
			srcIf:   "",
			wantErr: true,
		},
		{
			name:    "with description",
			addr:    "192.168.1.2",
			asn:     65002,
			srcIf:   "loopback0",
			opts:    []BGPPeerOption{WithDescription("test peer")},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			peer, err := NewBGPPeer(test.addr, test.asn, test.srcIf, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewBGPPeer() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewBGPPeer() unexpected error = %v", err)
				return
			}
			if peer == nil {
				t.Errorf("NewBGPPeer() returned nil BGPPeer instance")
				return
			}
			if peer.AsNumber != "65002" {
				t.Errorf("NewBGPPeer() AsNumber = %v, want %v", peer.AsNumber, "65002")
			}
			expectedIP := netip.MustParseAddr(test.addr)
			if peer.Addr != expectedIP {
				t.Errorf("NewBGPPeer() Addr = %v, want %v", peer.Addr, expectedIP)
			}
			if peer.SrcIf != test.srcIf {
				t.Errorf("NewBGPPeer() SrcIf = %v, want %v", peer.SrcIf, test.srcIf)
			}
		})
	}
}

func TestWithDescription(t *testing.T) {
	tests := []struct {
		name    string
		desc    string
		wantErr bool
	}{
		{
			name:    "valid description",
			desc:    "test peer description",
			wantErr: false,
		},
		{
			name:    "empty description",
			desc:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			peer := &BGPPeer{}
			opt := WithDescription(test.desc)
			err := opt(peer)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithDescription() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithDescription() unexpected error = %v", err)
				return
			}
			if peer.Desc != test.desc {
				t.Errorf("WithDescription() Desc = %v, want %v", peer.Desc, test.desc)
			}
		})
	}
}

func TestWithPeerAddressFamily(t *testing.T) {
	tests := []struct {
		name    string
		af      PeerAddressFamily
		wantErr bool
	}{
		{
			name:    "nil address family",
			af:      nil,
			wantErr: true,
		},
		{
			name:    "valid PeerL2EVPN",
			af:      &PeerL2EVPN{},
			wantErr: false,
		},
		{
			name:    "valid PeerIPv4Unicast",
			af:      &PeerIPv4Unicast{},
			wantErr: false,
		},
		{
			name:    "valid PeerIPv6Unicast",
			af:      &PeerIPv6Unicast{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			peer := &BGPPeer{}
			opt := WithPeerAddressFamily(test.af)
			err := opt(peer)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithPeerAddressFamily() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithPeerAddressFamily() unexpected error = %v", err)
				return
			}
			if len(peer.AddressFamilies) != 1 {
				t.Errorf("WithPeerAddressFamily() expected 1 address family, got %d", len(peer.AddressFamilies))
			}
		})
	}
}

func TestWithPeerAddressFamily_Duplicate(t *testing.T) {
	peer := &BGPPeer{}
	af1 := &PeerIPv4Unicast{}
	af2 := &PeerIPv4Unicast{}

	// Add first address family
	opt1 := WithPeerAddressFamily(af1)
	if err := opt1(peer); err != nil {
		t.Fatalf("WithPeerAddressFamily() unexpected error = %v", err)
	}

	// Try to add duplicate address family
	opt2 := WithPeerAddressFamily(af2)
	err := opt2(peer)
	if err == nil {
		t.Errorf("WithPeerAddressFamily() expected error for duplicate address family, got nil")
		return
	}
	expectedErr := "bgp peer: address family *bgp.PeerIPv4Unicast already exists"
	if err.Error() != expectedErr {
		t.Errorf("WithPeerAddressFamily() error = %v, want %v", err, expectedErr)
	}
}

func TestBGPPeer_ToYGOT_MissingBGPInstance(t *testing.T) {
	peer, err := NewBGPPeer("192.168.1.2", 65002, "loopback0")
	if err != nil {
		t.Fatalf("NewBGPPeer() unexpected error = %v", err)
	}

	client := &gnmiext.ClientMock{
		GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
			return gnmiext.ErrNil
		},
	}

	_, err = peer.ToYGOT(t.Context(), client)
	if err == nil {
		t.Errorf("BGPPeer.ToYGOT() expected error for missing BGP instance, got nil")
		return
	}
	if !errors.Is(err, ErrMissingBGPInstance) {
		t.Errorf("BGPPeer.ToYGOT() error = %v, want %v", err, ErrMissingBGPInstance)
	}
}

func TestBGPPeer_ToYGOT_GetError(t *testing.T) {
	peer, err := NewBGPPeer("192.168.1.2", 65002, "loopback0")
	if err != nil {
		t.Fatalf("NewBGPPeer() unexpected error = %v", err)
	}

	expectedErr := errors.New("get error")
	client := &gnmiext.ClientMock{
		GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
			return expectedErr
		},
	}

	_, err = peer.ToYGOT(t.Context(), client)
	if err == nil {
		t.Errorf("BGPPeer.ToYGOT() expected error, got nil")
		return
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("BGPPeer.ToYGOT() error = %v, want %v", err, expectedErr)
	}
}

func TestBGPPeer_ToYGOT_Success(t *testing.T) {
	peer, err := NewBGPPeer("192.168.1.2", 65002, "loopback0", WithDescription("test peer"))
	if err != nil {
		t.Fatalf("NewBGPPeer() unexpected error = %v", err)
	}

	client := &gnmiext.ClientMock{
		GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
			if xpath != "System/bgp-items/inst-items" {
				t.Errorf("BGPPeer.ToYGOT() unexpected xpath = %v", xpath)
			}
			// Simulate existing BGP instance
			inst := dest.(*nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems)
			inst.GetOrCreateDomItems().GetOrCreateDomList("default")
			return nil
		},
	}

	got, err := peer.ToYGOT(t.Context(), client)
	if err != nil {
		t.Errorf("BGPPeer.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 1 {
		t.Errorf("BGPPeer.ToYGOT() expected 1 update, got %d", len(got))
		return
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("BGPPeer.ToYGOT() expected update to be ReplacingUpdate")
		return
	}

	expectedXPath := "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=192.168.1.2]"
	if update.XPath != expectedXPath {
		t.Errorf("BGPPeer.ToYGOT() expected XPath %v, got %v", expectedXPath, update.XPath)
	}

	peerList, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList)
	if !ok {
		t.Errorf("BGPPeer.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList")
		return
	}

	if *peerList.Asn != "65002" {
		t.Errorf("BGPPeer.ToYGOT() expected ASN '65002', got %v", *peerList.Asn)
	}
	if peerList.AsnType != nxos.Cisco_NX_OSDevice_Bgp_PeerAsnType_none {
		t.Errorf("BGPPeer.ToYGOT() expected ASN type to be none")
	}
	if *peerList.Name != "test peer" {
		t.Errorf("BGPPeer.ToYGOT() expected name 'test peer', got %v", *peerList.Name)
	}
	if *peerList.SrcIf != "loopback0" {
		t.Errorf("BGPPeer.ToYGOT() expected source interface 'loopback0', got %v", *peerList.SrcIf)
	}
}

func TestBGPPeer_Reset(t *testing.T) {
	peer, err := NewBGPPeer("192.168.1.2", 65002, "loopback0")
	if err != nil {
		t.Fatalf("NewBGPPeer() unexpected error = %v", err)
	}

	got, err := peer.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("BGPPeer.Reset() unexpected error = %v", err)
		return
	}

	if len(got) != 1 {
		t.Errorf("BGPPeer.Reset() expected 1 update, got %d", len(got))
		return
	}

	update, ok := got[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("BGPPeer.Reset() expected update to be DeletingUpdate")
		return
	}

	expectedXPath := "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=192.168.1.2]"
	if update.XPath != expectedXPath {
		t.Errorf("BGPPeer.Reset() expected XPath %v, got %v", expectedXPath, update.XPath)
	}
}

func TestPeerAddressFamily_toYGOT(t *testing.T) {
	tests := []struct {
		name string
		af   PeerAddressFamily
		want *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList
	}{
		{
			name: "default PeerL2EVPN",
			af:   &PeerL2EVPN{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				Ctrl:       ygot.String("DME_UNSET_PROPERTY_MARKER"),
			},
		},
		{
			name: "PeerL2EVPN with standard community",
			af:   &PeerL2EVPN{SendStandardCommunity: true},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				Ctrl:       ygot.String("DME_UNSET_PROPERTY_MARKER"),
			},
		},
		{
			name: "PeerL2EVPN with extended community",
			af:   &PeerL2EVPN{SendExtendedCommunity: true},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				Ctrl:       ygot.String("DME_UNSET_PROPERTY_MARKER"),
			},
		},
		{
			name: "PeerL2EVPN with both communities",
			af:   &PeerL2EVPN{SendStandardCommunity: true, SendExtendedCommunity: true},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				Ctrl:       ygot.String("DME_UNSET_PROPERTY_MARKER"),
			},
		},
		{
			name: "PeerL2EVPN with route reflector client",
			af:   &PeerL2EVPN{RouteReflectorClient: true},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled,
				Ctrl:       ygot.String("rr-client"),
			},
		},
		{
			name: "PeerL2EVPN with all options",
			af:   &PeerL2EVPN{SendStandardCommunity: true, SendExtendedCommunity: true, RouteReflectorClient: true},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type:       nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn,
				SendComStd: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				SendComExt: nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled,
				Ctrl:       ygot.String("rr-client"),
			},
		},
		{
			name: "PeerIPv4Unicast",
			af:   &PeerIPv4Unicast{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type: nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast,
			},
		},
		{
			name: "PeerIPv6Unicast",
			af:   &PeerIPv6Unicast{},
			want: &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{
				Type: nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.af.toYGOT()
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("%T.toYGOT() = %+v, want %+v", test.af, got, test.want)
			}
		})
	}
}
