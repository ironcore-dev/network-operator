// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pim

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestNewRendezvousPoint(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		opts    []RendezvousPointOption
		wantErr bool
		wantVrf string
	}{
		{
			name:    "valid rendezvous point",
			addr:    "192.168.1.1",
			wantErr: false,
			wantVrf: "default",
		},
		{
			name:    "empty address",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "invalid address format",
			addr:    "invalid-ip",
			wantErr: true,
		},
		{
			name:    "IPv6 address",
			addr:    "2001:db8::1",
			wantErr: true,
		},
		{
			name:    "with group list",
			addr:    "192.168.1.1",
			opts:    []RendezvousPointOption{WithGroupList("224.0.0.0/4")},
			wantErr: false,
			wantVrf: "default",
		},
		{
			name:    "with custom VRF",
			addr:    "192.168.1.1",
			opts:    []RendezvousPointOption{WithRendezvousPointVRF("management")},
			wantErr: false,
			wantVrf: "management",
		},
		{
			name:    "with multiple options",
			addr:    "192.168.1.1",
			opts:    []RendezvousPointOption{WithGroupList("224.0.0.0/4"), WithRendezvousPointVRF("custom")},
			wantErr: false,
			wantVrf: "custom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rp, err := NewRendezvousPoint(test.addr, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewRendezvousPoint() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewRendezvousPoint() unexpected error = %v", err)
				return
			}
			if rp == nil {
				t.Errorf("NewRendezvousPoint() returned nil")
				return
			}
			if rp.Addr.String() != test.addr {
				t.Errorf("NewRendezvousPoint() Addr = %v, want %v", rp.Addr, test.addr)
			}
			if rp.Vrf != test.wantVrf {
				t.Errorf("NewRendezvousPoint() Vrf = %v, want %v", rp.Vrf, test.wantVrf)
			}
		})
	}
}

func TestWithGroupList(t *testing.T) {
	tests := []struct {
		name    string
		group   string
		wantErr bool
	}{
		{
			name:    "valid group list",
			group:   "224.0.0.0/4",
			wantErr: false,
		},
		{
			name:    "empty group list",
			group:   "",
			wantErr: true,
		},
		{
			name:    "invalid group format",
			group:   "invalid-prefix",
			wantErr: true,
		},
		{
			name:    "IPv6 group",
			group:   "ff00::/8",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rp := &RendezvousPoint{}
			opt := WithGroupList(test.group)
			err := opt(rp)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithGroupList() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithGroupList() unexpected error = %v", err)
				return
			}
			if rp.Group.String() != test.group {
				t.Errorf("WithGroupList() Group = %v, want %v", rp.Group, test.group)
			}
		})
	}
}

func TestWithRendezvousPointVRF(t *testing.T) {
	tests := []struct {
		name    string
		vrf     string
		wantErr bool
	}{
		{
			name:    "valid VRF",
			vrf:     "management",
			wantErr: false,
		},
		{
			name:    "empty VRF",
			vrf:     "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rp := &RendezvousPoint{}
			opt := WithRendezvousPointVRF(test.vrf)
			err := opt(rp)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithRendezvousPointVRF() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithRendezvousPointVRF() unexpected error = %v", err)
				return
			}
			if rp.Vrf != test.vrf {
				t.Errorf("WithRendezvousPointVRF() Vrf = %v, want %v", rp.Vrf, test.vrf)
			}
		})
	}
}

func TestRendezvousPoint_CIDR(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{
			name: "valid IPv4 address",
			addr: "192.168.1.1",
			want: "192.168.1.1/32",
		},
		{
			name:    "invalid address",
			addr:    "invalid",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rp, err := NewRendezvousPoint(test.addr)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewRendezvousPoint() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewRendezvousPoint() unexpected error = %v", err)
			}

			got, err := rp.CIDR()
			if err != nil {
				t.Errorf("RendezvousPoint.CIDR() unexpected error = %v", err)
				return
			}
			if got != test.want {
				t.Errorf("RendezvousPoint.CIDR() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRendezvousPoint_ToYGOT(t *testing.T) {
	rp, err := NewRendezvousPoint("192.168.1.1")
	if err != nil {
		t.Fatalf("NewRendezvousPoint() unexpected error = %v", err)
	}

	got, err := rp.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("RendezvousPoint.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("RendezvousPoint.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check PIM feature enablement
	update1, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected first update to be EditingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/pim-items" {
		t.Errorf("RendezvousPoint.ToYGOT() expected XPath 'System/fm-items/pim-items', got %v", update1.XPath)
	}

	pimItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_PimItems)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_PimItems")
		return
	}
	if pimItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("RendezvousPoint.ToYGOT() expected PIM AdminSt to be enabled")
	}

	// Check rendezvous point configuration
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items/StaticRP-list[addr=192.168.1.1/32]"
	if update2.XPath != expectedXPath {
		t.Errorf("RendezvousPoint.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	rpList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList")
		return
	}

	if *rpList.Addr != "192.168.1.1/32" {
		t.Errorf("RendezvousPoint.ToYGOT() expected Addr '192.168.1.1/32', got %v", *rpList.Addr)
	}
}

func TestRendezvousPoint_ToYGOT_WithGroup(t *testing.T) {
	rp, err := NewRendezvousPoint("192.168.1.1", WithGroupList("224.0.0.0/4"))
	if err != nil {
		t.Fatalf("NewRendezvousPoint() unexpected error = %v", err)
	}

	got, err := rp.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("RendezvousPoint.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("RendezvousPoint.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check rendezvous point configuration with group
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}

	rpList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList")
		return
	}

	// Check that group list is configured
	if rpList.RpgrplistItems == nil {
		t.Errorf("RendezvousPoint.ToYGOT() expected RpgrplistItems to be configured")
		return
	}

	groupList := rpList.RpgrplistItems.GetRPGrpListList("224.0.0.0/4")
	if groupList == nil {
		t.Errorf("RendezvousPoint.ToYGOT() expected group list '224.0.0.0/4' to be present")
		return
	}

	if *groupList.Bidir != false {
		t.Errorf("RendezvousPoint.ToYGOT() expected Bidir to be false")
	}
	if *groupList.Override != false {
		t.Errorf("RendezvousPoint.ToYGOT() expected Override to be false")
	}
}

func TestRendezvousPoint_ToYGOT_WithCustomVRF(t *testing.T) {
	rp, err := NewRendezvousPoint("192.168.1.1", WithRendezvousPointVRF("management"))
	if err != nil {
		t.Fatalf("NewRendezvousPoint() unexpected error = %v", err)
	}

	got, err := rp.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("RendezvousPoint.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("RendezvousPoint.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check rendezvous point configuration with custom VRF
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=management]/staticrp-items/rp-items/StaticRP-list[addr=192.168.1.1/32]"
	if update2.XPath != expectedXPath {
		t.Errorf("RendezvousPoint.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	rpList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList)
	if !ok {
		t.Errorf("RendezvousPoint.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList")
		return
	}

	if *rpList.Addr != "192.168.1.1/32" {
		t.Errorf("RendezvousPoint.ToYGOT() expected Addr '192.168.1.1/32', got %v", *rpList.Addr)
	}
}

func TestRendezvousPoint_Reset(t *testing.T) {
	addr, err := netip.ParseAddr("192.168.1.1")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	tests := []struct {
		name         string
		rp           *RendezvousPoint
		expectedPath string
	}{
		{
			name: "default VRF",
			rp: &RendezvousPoint{
				Addr: addr,
				Vrf:  "default",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=default]/staticrp-items/rp-items/StaticRP-list[addr=192.168.1.1/32]",
		},
		{
			name: "custom VRF",
			rp: &RendezvousPoint{
				Addr: addr,
				Vrf:  "management",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=management]/staticrp-items/rp-items/StaticRP-list[addr=192.168.1.1/32]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.rp.Reset(&gnmiext.ClientMock{})
			if err != nil {
				t.Errorf("RendezvousPoint.Reset() unexpected error = %v", err)
				return
			}

			if len(got) != 1 {
				t.Errorf("RendezvousPoint.Reset() expected 1 update, got %d", len(got))
				return
			}

			update, ok := got[0].(gnmiext.DeletingUpdate)
			if !ok {
				t.Errorf("RendezvousPoint.Reset() expected update to be DeletingUpdate")
				return
			}

			if update.XPath != test.expectedPath {
				t.Errorf("RendezvousPoint.Reset() expected XPath %v, got %v", test.expectedPath, update.XPath)
			}
		})
	}
}

func TestNewAnycastPeer(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		opts    []AnycastPeerOption
		wantErr bool
		wantVrf string
	}{
		{
			name:    "valid anycast peer",
			addr:    "192.168.1.2",
			wantErr: false,
			wantVrf: "default",
		},
		{
			name:    "empty address",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "invalid address format",
			addr:    "invalid-ip",
			wantErr: true,
		},
		{
			name:    "IPv6 address",
			addr:    "2001:db8::1",
			wantErr: true,
		},
		{
			name:    "with custom VRF",
			addr:    "192.168.1.2",
			opts:    []AnycastPeerOption{WithAnycastPeerVRF("management")},
			wantErr: false,
			wantVrf: "management",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap, err := NewAnycastPeer(test.addr, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewAnycastPeer() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewAnycastPeer() unexpected error = %v", err)
				return
			}
			if ap == nil {
				t.Errorf("NewAnycastPeer() returned nil")
				return
			}
			if ap.Addr.String() != test.addr {
				t.Errorf("NewAnycastPeer() Addr = %v, want %v", ap.Addr, test.addr)
			}
			if ap.Vrf != test.wantVrf {
				t.Errorf("NewAnycastPeer() Vrf = %v, want %v", ap.Vrf, test.wantVrf)
			}
		})
	}
}

func TestWithAnycastPeerVRF(t *testing.T) {
	tests := []struct {
		name    string
		vrf     string
		wantErr bool
	}{
		{
			name:    "valid VRF",
			vrf:     "management",
			wantErr: false,
		},
		{
			name:    "empty VRF",
			vrf:     "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap := &AnycastPeer{}
			opt := WithAnycastPeerVRF(test.vrf)
			err := opt(ap)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithAnycastPeerVRF() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithAnycastPeerVRF() unexpected error = %v", err)
				return
			}
			if ap.Vrf != test.vrf {
				t.Errorf("WithAnycastPeerVRF() Vrf = %v, want %v", ap.Vrf, test.vrf)
			}
		})
	}
}

func TestAnycastPeer_CIDR(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{
			name: "valid IPv4 address",
			addr: "192.168.1.2",
			want: "192.168.1.2/32",
		},
		{
			name:    "invalid address",
			addr:    "invalid",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap, err := NewAnycastPeer(test.addr)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewAnycastPeer() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewAnycastPeer() unexpected error = %v", err)
			}

			got, err := ap.CIDR()
			if err != nil {
				t.Errorf("AnycastPeer.CIDR() unexpected error = %v", err)
				return
			}
			if got != test.want {
				t.Errorf("AnycastPeer.CIDR() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAnycastPeer_ToYGOT(t *testing.T) {
	ap, err := NewAnycastPeer("192.168.1.2")
	if err != nil {
		t.Fatalf("NewAnycastPeer() unexpected error = %v", err)
	}

	got, err := ap.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("AnycastPeer.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("AnycastPeer.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check PIM feature enablement
	update1, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected first update to be EditingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/pim-items" {
		t.Errorf("AnycastPeer.ToYGOT() expected XPath 'System/fm-items/pim-items', got %v", update1.XPath)
	}

	pimItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_PimItems)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_PimItems")
		return
	}
	if pimItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("AnycastPeer.ToYGOT() expected PIM AdminSt to be enabled")
	}

	// Check anycast peer configuration
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=default]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=192.168.1.2/32][rpSetAddr=192.168.1.2/32]"
	if update2.XPath != expectedXPath {
		t.Errorf("AnycastPeer.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	peerList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_AcastrpfuncItems_PeerItems_AcastRPPeerList)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_AcastrpfuncItems_PeerItems_AcastRPPeerList")
		return
	}

	if *peerList.Addr != "192.168.1.2/32" {
		t.Errorf("AnycastPeer.ToYGOT() expected Addr '192.168.1.2/32', got %v", *peerList.Addr)
	}
	if *peerList.RpSetAddr != "192.168.1.2/32" {
		t.Errorf("AnycastPeer.ToYGOT() expected RpSetAddr '192.168.1.2/32', got %v", *peerList.RpSetAddr)
	}
}

func TestAnycastPeer_ToYGOT_WithCustomVRF(t *testing.T) {
	ap, err := NewAnycastPeer("192.168.1.2", WithAnycastPeerVRF("management"))
	if err != nil {
		t.Fatalf("NewAnycastPeer() unexpected error = %v", err)
	}

	got, err := ap.ToYGOT(&gnmiext.ClientMock{
		ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Errorf("AnycastPeer.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("AnycastPeer.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check anycast peer configuration with custom VRF
	update2, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected second update to be ReplacingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=management]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=192.168.1.2/32][rpSetAddr=192.168.1.2/32]"
	if update2.XPath != expectedXPath {
		t.Errorf("AnycastPeer.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	peerList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_AcastrpfuncItems_PeerItems_AcastRPPeerList)
	if !ok {
		t.Errorf("AnycastPeer.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_AcastrpfuncItems_PeerItems_AcastRPPeerList")
		return
	}

	if *peerList.Addr != "192.168.1.2/32" {
		t.Errorf("AnycastPeer.ToYGOT() expected Addr '192.168.1.2/32', got %v", *peerList.Addr)
	}
	if *peerList.RpSetAddr != "192.168.1.2/32" {
		t.Errorf("AnycastPeer.ToYGOT() expected RpSetAddr '192.168.1.2/32', got %v", *peerList.RpSetAddr)
	}
}

func TestAnycastPeer_Reset(t *testing.T) {
	tests := []struct {
		name         string
		ap           *AnycastPeer
		expectedPath string
	}{
		{
			name: "default VRF",
			ap: &AnycastPeer{
				Addr: mustParseAddr("192.168.1.2"),
				Vrf:  "default",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=default]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=192.168.1.2/32][rpSetAddr=192.168.1.2/32]",
		},
		{
			name: "custom VRF",
			ap: &AnycastPeer{
				Addr: mustParseAddr("192.168.1.2"),
				Vrf:  "management",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=management]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=192.168.1.2/32][rpSetAddr=192.168.1.2/32]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.ap.Reset(&gnmiext.ClientMock{})
			if err != nil {
				t.Errorf("AnycastPeer.Reset() unexpected error = %v", err)
				return
			}

			if len(got) != 1 {
				t.Errorf("AnycastPeer.Reset() expected 1 update, got %d", len(got))
				return
			}

			update, ok := got[0].(gnmiext.DeletingUpdate)
			if !ok {
				t.Errorf("AnycastPeer.Reset() expected update to be DeletingUpdate")
				return
			}

			if update.XPath != test.expectedPath {
				t.Errorf("AnycastPeer.Reset() expected XPath %v, got %v", test.expectedPath, update.XPath)
			}
		})
	}
}

// Helper function to parse addresses for test cases
func mustParseAddr(addr string) netip.Addr {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		panic(err)
	}
	return a
}

func TestNewInterface(t *testing.T) {
	tests := []struct {
		name    string
		ifName  string
		opts    []InterfaceOption
		wantErr bool
		wantVrf string
	}{
		{
			name:    "with sparse mode enabled",
			ifName:  "lo0",
			opts:    []InterfaceOption{WithSparseMode(true)},
			wantErr: false,
			wantVrf: "default",
		},
		{
			name:    "with sparse mode disabled",
			ifName:  "lo0",
			opts:    []InterfaceOption{WithSparseMode(false)},
			wantErr: false,
			wantVrf: "default",
		},
		{
			name:    "with custom VRF",
			ifName:  "lo0",
			opts:    []InterfaceOption{WithInterfaceVRF("management")},
			wantErr: false,
			wantVrf: "management",
		},
		{
			name:    "with multiple options",
			ifName:  "eth1/1",
			opts:    []InterfaceOption{WithSparseMode(true), WithInterfaceVRF("custom")},
			wantErr: false,
			wantVrf: "custom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intf, err := NewInterface(test.ifName, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewInterface() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewInterface() unexpected error = %v", err)
				return
			}
			if intf == nil {
				t.Errorf("NewInterface() returned nil")
				return
			}
			if intf.Vrf != test.wantVrf {
				t.Errorf("NewInterface() Vrf = %v, want %v", intf.Vrf, test.wantVrf)
			}
		})
	}
}

func TestWithSparseMode(t *testing.T) {
	tests := []struct {
		name   string
		enable bool
	}{
		{
			name:   "enable sparse mode",
			enable: true,
		},
		{
			name:   "disable sparse mode",
			enable: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intf := &Interface{}
			opt := WithSparseMode(test.enable)
			err := opt(intf)
			if err != nil {
				t.Errorf("WithSparseMode() unexpected error = %v", err)
				return
			}
			if intf.SparseMode != test.enable {
				t.Errorf("WithSparseMode() SparseMode = %v, want %v", intf.SparseMode, test.enable)
			}
		})
	}
}

func TestWithInterfaceVRF(t *testing.T) {
	tests := []struct {
		name    string
		vrf     string
		wantErr bool
	}{
		{
			name:    "valid VRF",
			vrf:     "management",
			wantErr: false,
		},
		{
			name:    "empty VRF",
			vrf:     "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intf := &Interface{}
			opt := WithInterfaceVRF(test.vrf)
			err := opt(intf)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithInterfaceVRF() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithInterfaceVRF() unexpected error = %v", err)
				return
			}
			if intf.Vrf != test.vrf {
				t.Errorf("WithInterfaceVRF() Vrf = %v, want %v", intf.Vrf, test.vrf)
			}
		})
	}
}

func TestInterface_ToYGOT_MissingInterface(t *testing.T) {
	intf, err := NewInterface("lo0")
	if err != nil {
		t.Fatalf("NewInterface() unexpected error = %v", err)
	}

	client := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			return false, nil
		},
	}

	_, err = intf.ToYGOT(client)
	if err == nil {
		t.Errorf("Interface.ToYGOT() expected error for missing interface, got nil")
		return
	}
	if !errors.Is(err, ErrMissingInterface) {
		t.Errorf("Interface.ToYGOT() error = %v, want %v", err, ErrMissingInterface)
	}
}

func TestInterface_ToYGOT_ExistsError(t *testing.T) {
	intf, err := NewInterface("lo0")
	if err != nil {
		t.Fatalf("NewInterface() unexpected error = %v", err)
	}

	expectedErr := errors.New("get error")
	client := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			return false, expectedErr
		},
	}

	_, err = intf.ToYGOT(client)
	if err == nil {
		t.Errorf("Interface.ToYGOT() expected error, got nil")
	}
}

func TestInterface_ToYGOT_Success(t *testing.T) {
	intf, err := NewInterface("lo0", WithSparseMode(true))
	if err != nil {
		t.Fatalf("NewInterface() unexpected error = %v", err)
	}

	client := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			if xpath != "System/intf-items/lb-items/LbRtdIf-list[id=lo0]" {
				t.Errorf("Interface.ToYGOT() unexpected xpath = %v", xpath)
			}
			// Simulate existing interface
			return true, nil
		},
	}

	got, err := intf.ToYGOT(client)
	if err != nil {
		t.Errorf("Interface.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("Interface.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check PIM feature enablement
	update1, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected first update to be EditingUpdate")
		return
	}
	if update1.XPath != "System/fm-items/pim-items" {
		t.Errorf("Interface.ToYGOT() expected XPath 'System/fm-items/pim-items', got %v", update1.XPath)
	}

	pimItems, ok := update1.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_PimItems)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_FmItems_PimItems")
		return
	}
	if pimItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("Interface.ToYGOT() expected PIM AdminSt to be enabled")
	}

	// Check interface configuration
	update2, ok := got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected second update to be EditingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=" + intf.Vrf + "]/if-items/If-list[id=" + intf.Name + "]"
	if update2.XPath != expectedXPath {
		t.Errorf("Interface.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	ifList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList")
		return
	}

	if *ifList.Ctrl != "border" {
		t.Errorf("Interface.ToYGOT() expected Ctrl 'border', got %v", *ifList.Ctrl)
	}
	if *ifList.PimSparseMode != true {
		t.Errorf("Interface.ToYGOT() expected PimSparseMode true, got %v", *ifList.PimSparseMode)
	}
}

func TestInterface_ToYGOT_WithCustomVRF(t *testing.T) {
	intf, err := NewInterface("eth1/1", WithSparseMode(false), WithInterfaceVRF("management"))
	if err != nil {
		t.Fatalf("NewInterface() unexpected error = %v", err)
	}

	client := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			if xpath != "System/intf-items/phys-items/PhysIf-list[id=eth1/1]" {
				t.Errorf("Interface.ToYGOT() unexpected xpath = %v", xpath)
			}
			// Simulate existing interface
			return true, nil
		},
	}

	got, err := intf.ToYGOT(client)
	if err != nil {
		t.Errorf("Interface.ToYGOT() unexpected error = %v", err)
		return
	}

	if len(got) != 2 {
		t.Errorf("Interface.ToYGOT() expected 2 updates, got %d", len(got))
		return
	}

	// Check interface configuration with custom VRF
	update2, ok := got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected second update to be EditingUpdate")
		return
	}

	expectedXPath := "System/pim-items/inst-items/dom-items/Dom-list[name=management]/if-items/If-list[id=eth1/1]"
	if update2.XPath != expectedXPath {
		t.Errorf("Interface.ToYGOT() expected XPath %v, got %v", expectedXPath, update2.XPath)
	}

	ifList, ok := update2.Value.(*nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList)
	if !ok {
		t.Errorf("Interface.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList")
		return
	}

	if *ifList.Ctrl != "border" {
		t.Errorf("Interface.ToYGOT() expected Ctrl 'border', got %v", *ifList.Ctrl)
	}
	if *ifList.PimSparseMode != false {
		t.Errorf("Interface.ToYGOT() expected PimSparseMode false, got %v", *ifList.PimSparseMode)
	}
}

func TestInterface_Reset(t *testing.T) {
	tests := []struct {
		name         string
		intf         *Interface
		expectedPath string
	}{
		{
			name: "default VRF",
			intf: &Interface{
				Name: "lo0",
				Vrf:  "default",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=lo0]",
		},
		{
			name: "custom VRF",
			intf: &Interface{
				Name: "eth1/1",
				Vrf:  "management",
			},
			expectedPath: "System/pim-items/inst-items/dom-items/Dom-list[name=management]/if-items/If-list[id=eth1/1]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.intf.Reset(&gnmiext.ClientMock{})
			if err != nil {
				t.Errorf("Interface.Reset() unexpected error = %v", err)
				return
			}

			if len(got) != 1 {
				t.Errorf("Interface.Reset() expected 1 update, got %d", len(got))
				return
			}

			update, ok := got[0].(gnmiext.DeletingUpdate)
			if !ok {
				t.Errorf("Interface.Reset() expected update to be DeletingUpdate")
				return
			}

			if update.XPath != test.expectedPath {
				t.Errorf("Interface.Reset() expected XPath %v, got %v", test.expectedPath, update.XPath)
			}
		})
	}
}
