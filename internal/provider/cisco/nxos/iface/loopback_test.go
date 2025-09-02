// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

const (
	loopbackName        = "Loopback0"
	loopbackShortName   = "lo0"
	loopbackDescription = "Test Loopback Interface"
	loopbackVRFName     = "test-vrf"
)

func Test_NewLoopback(t *testing.T) {
	validNames := []string{"Loopback0", "loopback123", "lo1", "lo99"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			_, err := NewLoopbackInterface(name, nil)
			if err != nil {
				t.Fatalf("failed to create physical interface: %v", err)
			}
		})
	}
	invalidNames := []string{"test", "Loopback", "lo", "Loopback1/2", "lo1.1", "eth100"}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := NewLoopbackInterface(name, nil)
			if err == nil {
				t.Fatalf("created interface with invalid name: %s", name)
			}
		})
	}
}

func Test_Loopback_ToYGOT_BaseConfig(t *testing.T) {
	t.Run("No additional base options", func(t *testing.T) {
		p, err := NewLoopbackInterface(loopbackName, ygot.String(loopbackDescription))
		if err != nil {
			t.Fatalf("failed to create loopback interface: %v", err)
		}

		got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// single update affecting only base configuration of physical interface
		if len(got) != 1 {
			t.Errorf("expected 2 update, got %d", len(got))
		}
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/lb-items/LbRtdIf-list[id="+p.name+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/lb-items/LbRtdIf-list[id=%s]', got '%s'", p.name, bUpdate.XPath)
		}
		// correct initialization
		phRef := &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
			Descr:   ygot.String(loopbackDescription),
			AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
		}
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList)
		notification, err := ygot.Diff(phGot, phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	t.Run("With VRF", func(t *testing.T) {
		p, err := NewLoopbackInterface(loopbackName, ygot.String(loopbackDescription), WithLoopbackVRF("test-vrf"))
		if err != nil {
			t.Fatalf("failed to create loopback interface: %v", err)
		}
		got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 update, got %d", len(got))
		}
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/lb-items/LbRtdIf-list[id="+p.name+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/lb-items/LbRtdIf-list[id=%s]', got '%s'", p.name, bUpdate.XPath)
		}

		llRef := &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
			Descr:   ygot.String(loopbackDescription),
			AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList_RtvrfMbrItems{
				TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
			},
		}
		llGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList)
		notification, err := ygot.Diff(llGot, llRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
}

func Test_Loopback_ToYGOT_WithL3Config(t *testing.T) {
	l3cfg, err := NewL3Config(
		WithNumberedAddressingIPv4([]string{"10.0.0.1/24"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, err := NewLoopbackInterface(loopbackName, ygot.String(loopbackDescription),
		WithLoopbackL3(l3cfg),
		WithLoopbackVRF("test-vrf"),
	)
	if err != nil {
		t.Fatalf("failed to create loopback interface: %v", err)
	}
	got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 updates (base + L3), got %d", len(got))
	}

	t.Run("Addressing", func(t *testing.T) {
		aUpdate := got[1].(gnmiext.ReplacingUpdate)
		if aUpdate.XPath != "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id="+loopbackShortName+"]" {
			t.Errorf("wrong xpath, expected 'System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id="+loopbackShortName+"]', got '%s'", aUpdate.XPath)
		}
		expected := &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
			AddrItems: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
				AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
					"10.0.0.1/24": {
						Addr: ygot.String("10.0.0.1/24"),
					},
				},
			},
		}
		aGot := aUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList)
		notification, err := ygot.Diff(aGot, expected)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
}

func Test_Loopback_ToYGOT_InvalidL3Config(t *testing.T) {
	t.Run("With unnumbered addressing", func(t *testing.T) {
		l3cfg, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
			WithUnnumberedAddressing("loopback1"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = NewLoopbackInterface(loopbackName, ygot.String(loopbackDescription), WithLoopbackL3(l3cfg))
		if err == nil {
			t.Fatalf("expected error for unnumbered addressing, got nil")
		}
	})
	t.Run("With medium ", func(t *testing.T) {
		l3cfg, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = NewLoopbackInterface(loopbackName, ygot.String(loopbackDescription), WithLoopbackL3(l3cfg))
		if err == nil {
			t.Fatalf("expected error for medium type, got nil")
		}
	})
}
