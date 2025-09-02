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
	physIfDescription = "test interface"
	physIfVRFName     = "test-vrf"
	physIfName        = "eth1/1"
)

func Test_PhysIf_NewPhysicalInterface(t *testing.T) {
	validNames := []string{"Ethernet1/1", "ethernet1/2", "eth1/1", "eTH1/2", "Eth1/3"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			_, err := NewPhysicalInterface(name)
			if err != nil {
				t.Fatalf("failed to create physical interface: %v", err)
			}
		})
	}
	invalidNames := []string{"test", "ether1/1", "ethernet1.1", "eth1/1/1", "port-channel01", "po100"}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := NewPhysicalInterface(name)
			if err == nil {
				t.Fatalf("created interface with invalid name: %s", name)
			}
		})
	}
}

// tests base configuration of the physical interface is correctly initialized
func Test_PhysIf_ToYGOT_BaseConfig(t *testing.T) {
	t.Run("No additional base options", func(t *testing.T) {
		p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription))
		if err != nil {
			t.Fatalf("failed to create physical interface")
		}

		got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// single update affecting only base configuration of physical interface
		if len(got) != 1 {
			t.Errorf("expected 1 update, got %d", len(got))
		}
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/phys-items/PhysIf-list[id="+p.name+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/phys-items/PhysIf-list[id=%s]', got '%s'", p.name, bUpdate.XPath)
		}

		// correct initialization
		phRef := &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			Descr:         ygot.String(physIfDescription),
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			UserCfgdFlags: ygot.String("admin_state"),
		}
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
	t.Run("MTU and VRF", func(t *testing.T) {
		p, err := NewPhysicalInterface(
			physIfName,
			WithDescription(physIfDescription),
			WithPhysIfMTU(9216),
			WithPhysIfVRF(physIfVRFName),
		)
		if err != nil {
			t.Fatalf("failed to create physical interface")
		}

		got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// single update affecting only base configuration of physical interface
		if len(got) != 1 {
			t.Errorf("expected 1 update, got %d", len(got))
		}
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/phys-items/PhysIf-list[id="+p.name+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/phys-items/PhysIf-list[id=%s]', got '%s'", p.name, bUpdate.XPath)
		}

		// correct initialization
		phRef := &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			Descr:         ygot.String(physIfDescription),
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			Mtu:           ygot.Uint32(9216),
			UserCfgdFlags: ygot.String("admin_mtu,admin_state"),
		}
		phRef.GetOrCreateRtvrfMbrItems().TDn = ygot.String("System/inst-items/Inst-list[name=" + physIfVRFName + "]")
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
}

func Test_PhysIf_Reset_BaseConfig(t *testing.T) {
	p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription))
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}

	got, err := p.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// expect 2 updates: base config and spanning tree
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}
	t.Run("Base config", func(t *testing.T) {
		// checks on base config update
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type EditingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/phys-items/PhysIf-list[id="+p.name+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/phys-items/PhysIf-list[id=%s]', got '%s'", p.name, bUpdate.XPath)
		}
		phRef := nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{}
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, &phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
}

// Test_PhysIf_ToYGOT_WithL2AndL3 verifies that when both L2 and L3 options are supplied,
// only the last one is applied, per contract.
func Test_PhysIf_ToYGOT_WithL2AndL3(t *testing.T) {
	l2cfg, err := NewL2Config()
	if err != nil {
		t.Fatalf("unexpected error while creating L2 config: %v", err)
	}
	l3cfg, err := NewL3Config()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("L2 with VRF is not allowed", func(t *testing.T) {
		_, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL2(l2cfg), WithPhysIfVRF(physIfVRFName))
		if err == nil {
			t.Fatalf("expected error when creating physical interface with L2 and VRF, got nil")
		}
	})
	t.Run("L2 then L3, expect only L3", func(t *testing.T) {
		p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL2(l2cfg), WithPhysIfL3(l3cfg))
		if err != nil {
			t.Fatalf("failed to create physical interface")
		}
		if p.l2 != nil {
			t.Errorf("expected L2 to be nil")
		}
		if p.l3 == nil {
			t.Errorf("expected L3 to be set")
		}
	})

	t.Run("L3 then L2, expect only L2", func(t *testing.T) {
		p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL3(l3cfg), WithPhysIfL2(l2cfg))
		if err != nil {
			t.Fatalf("failed to create physical interface")
		}
		if p.l2 == nil {
			t.Errorf("expected L2 to be set")
		}
		if p.l3 != nil {
			t.Errorf("expected L3 to be nil")
		}
	})
}

func Test_PhysIf_ToYGOT_WithL2_Trunk(t *testing.T) {
	l2cfg, err := NewL2Config(
		WithSpanningTree(SpanningTreeModeEdge),
		WithSwithPortMode(SwitchPortModeTrunk),
		WithNativeVlan(100),
		WithAllowedVlans([]uint16{10, 20, 30}),
	)
	if err != nil {
		t.Fatalf("unexpected error while creating L2 config: %v", err)
	}
	p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL2(l2cfg))
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}
	got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}
	t.Run("Base config", func(t *testing.T) {
		// check base config: additional layer option and switchport mode
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.Value == nil {
			t.Errorf("expected value to be set")
		}
		phRef := nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			Descr:         ygot.String(physIfDescription),
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
			Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_trunk,
			NativeVlan:    ygot.String("vlan-100"), // required by NX-OS
			TrunkVlans:    ygot.String("10,20,30"),
			UserCfgdFlags: ygot.String("admin_layer,admin_state"),
		}
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, &phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	// check l2 config: a single update for spanning tree
	t.Run("Spanning tree config", func(t *testing.T) {
		l2Update, ok := got[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		expectPath := "System/stp-items/inst-items/if-items/If-list[id=" + p.name + "]"
		if l2Update.XPath != expectPath {
			t.Errorf("wrong xpath, expected '%s', got '%s'", expectPath, l2Update.XPath)
		}
		stRef := nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
			AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
			Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_edge,
		}
		stGot := l2Update.Value.(*nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList)
		if *stGot != stRef {
			t.Errorf("spanning tree config mismatch")
		}
	})
}

func Test_PhysIf_ToYGOT_WithL2_Access(t *testing.T) {
	l2cfg, err := NewL2Config(
		WithSpanningTree(SpanningTreeModeEdge),
		WithSwithPortMode(SwitchPortModeAccess),
		WithAccessVlan(10),
	)
	if err != nil {
		t.Fatalf("unexpected error while creating L2 config: %v", err)
	}
	p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL2(l2cfg))
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}
	got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}
	t.Run("Base config", func(t *testing.T) {
		// check base config: additional layer option and switchport mode
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.Value == nil {
			t.Errorf("expected value to be set")
		}
		phRef := nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			Descr:         ygot.String(physIfDescription),
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
			Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_access,
			AccessVlan:    ygot.String("vlan-10"),
			UserCfgdFlags: ygot.String("admin_layer,admin_state"),
		}
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, &phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	// check l2 config: a single update for spanning tree
	t.Run("Spanning tree config", func(t *testing.T) {
		l2Update, ok := got[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		expectPath := "System/stp-items/inst-items/if-items/If-list[id=" + p.name + "]"
		if l2Update.XPath != expectPath {
			t.Errorf("wrong xpath, expected '%s', got '%s'", expectPath, l2Update.XPath)
		}
		stRef := nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
			AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
			Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_edge,
		}
		stGot := l2Update.Value.(*nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList)
		if *stGot != stRef {
			t.Errorf("spanning tree config mismatch")
		}
	})
}

func Test_PhysIf_ToYGOT_WithL3(t *testing.T) {
	l3cfg, err := NewL3Config(
		WithMedium(L3MediumTypeP2P),
		WithUnnumberedAddressing("loopback0"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL3(l3cfg))
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}
	got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}

	t.Run("Base config", func(t *testing.T) {
		// check base config: additional layer option and switchport mode
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.Value == nil {
			t.Errorf("expected value to be set")
		}
		phExpect := nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			Descr:         ygot.String(physIfDescription),
			Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
			Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
			UserCfgdFlags: ygot.String("admin_layer,admin_state"),
		}
		ph := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(ph, &phExpect)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	t.Run("Addressing config: unnumbered loopback0", func(t *testing.T) {
		// check addressing config: a single update for addressing
		aUpdate, ok := got[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		expectPath := "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=" + p.name + "]"
		if aUpdate.XPath != expectPath {
			t.Errorf("wrong xpath, expected '%s', got '%s'", expectPath, aUpdate.XPath)
		}
		addrRef := nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
			Unnumbered: ygot.String("lo0"),
		}
		addrGot := aUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList)
		notification, err := ygot.Diff(&addrRef, addrGot)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	// numbered addressing
	t.Run("Addressing config: numbered", func(t *testing.T) {
		l3cfg, err := NewL3Config(
			WithNumberedAddressingIPv4([]string{"192.0.2.1/8"}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p2, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL3(l3cfg))
		if err != nil {
			t.Fatalf("failed to create physical interface")
		}
		got, err := p2.ToYGOT(t.Context(), &gnmiext.ClientMock{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		aUpdate, ok := got[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		addrGot := aUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList)
		if addrGot.Unnumbered != nil {
			t.Errorf("expected unnumbered to be nil")
		}
		if addrGot.GetAddrItems().GetAddrList("192.0.2.1/8") == nil {
			t.Errorf("address is not set")
		}
	})
}

func Test_PhysIf_Reset_WithL3(t *testing.T) {
	l3cfg, err := NewL3Config(
		WithMedium(L3MediumTypeP2P),
		WithUnnumberedAddressing("loopback0"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p, err := NewPhysicalInterface(physIfName, WithDescription(physIfDescription), WithPhysIfL3(l3cfg))
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}
	got, err := p.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// expect 2 updates: base config and L2 spanning tree reset, which is not automatically reset. The L3 config
	// is automatically removed when resetting the physical interface.
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}
	t.Run("Base config", func(t *testing.T) {
		// check base config: additional layer option and switchport mode
		phUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		phGot := phUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		phExpect := nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{}
		notification, err := ygot.Diff(&phExpect, phGot)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})
}

// Configuring a VRF on an interface influences the xpath in several updates
func Test_PhysIf_ToYGOT_VRF(t *testing.T) {
	l3cfg, err := NewL3Config(
		WithMedium(L3MediumTypeP2P),
		WithUnnumberedAddressing("loopback0"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, err := NewPhysicalInterface(
		physIfName,
		WithDescription(physIfDescription),
		WithPhysIfVRF(physIfVRFName),
		WithPhysIfL3(l3cfg),
	)
	if err != nil {
		t.Fatalf("failed to create physical interface")
	}

	got, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// single update affecting only base configuration of physical interface
	if len(got) != 2 {
		t.Errorf("expected 2 update, got %d", len(got))
	}

	t.Run("Base config", func(t *testing.T) {
		bUpdate, ok := got[0].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Errorf("expected value to be of type ReplacingUpdate")
		}
		if bUpdate.XPath != "System/intf-items/phys-items/PhysIf-list[id="+physIfName+"]" {
			t.Errorf("wrong xpath, expected 'System/intf-items/phys-items/PhysIf-list[id="+physIfName+"]', got '%s'", bUpdate.XPath)
		}

		phRef := &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
			Descr:         ygot.String(physIfDescription),
			AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
			Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
			Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
			UserCfgdFlags: ygot.String("admin_layer,admin_state"),
		}
		phRef.GetOrCreateRtvrfMbrItems().TDn = ygot.String("System/inst-items/Inst-list[name=" + physIfVRFName + "]")
		phGot := bUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
		notification, err := ygot.Diff(phGot, phRef)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff: %s", notification)
		}
	})

	t.Run("Addressing", func(t *testing.T) {
		aUpdate := got[1].(gnmiext.ReplacingUpdate)
		if aUpdate.XPath != "System/ipv4-items/inst-items/dom-items/Dom-list[name="+physIfVRFName+"]/if-items/If-list[id="+physIfName+"]" {
			t.Errorf("wrong xpath, expected 'System/ipv4-items/inst-items/dom-items/Dom-list[name="+physIfVRFName+"]/if-items/If-list[id="+physIfName+"]', got '%s'", aUpdate.XPath)
		}
	})
}
