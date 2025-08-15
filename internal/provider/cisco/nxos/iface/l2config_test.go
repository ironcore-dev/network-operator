// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iface

import (
	"reflect"
	"testing"
)

func TestL2Config_AccessModeOptions(t *testing.T) {
	// valid vlan
	l2, err := NewL2Config(
		WithSwithPortMode(SwitchPortModeAccess),
		WithAccessVlan(100),
	)
	if err != nil {
		t.Fatalf("unexpected error while creating L2 config: %v", err)
	}
	if l2.accessVlan == nil || *l2.accessVlan != 100 {
		t.Errorf("expected accessVlan to be 100, got %v", l2.accessVlan)
	}

	// invalid VLAN
	l2, err = NewL2Config(WithSwithPortMode(SwitchPortModeAccess))
	if err != nil {
		t.Fatalf("unexpected error while creating L2 config: %v", err)
	}
	err = WithAccessVlan(0)(l2)
	if err == nil {
		t.Error("expected error for VLAN < 1")
	}
	err = WithAccessVlan(4095)(l2)
	if err == nil {
		t.Error("expected error for VLAN > 4094")
	}

	// use of trunk mode options in access mode
	_, err = NewL2Config(
		WithSwithPortMode(SwitchPortModeAccess),
		WithNativeVlan(200),
		WithAllowedVlans([]uint16{10, 20, 30}),
	)
	if err == nil {
		t.Fatalf("misconfig error not triggered wile using trunk options")
	}
}

func TestL2Config_TrunkModeOptions(t *testing.T) {
	// trunk mode with NativeVlan and AllowedVlans
	t.Run("Valid trunk mode with configured native and allowed vlans", func(t *testing.T) {
		l2, err := NewL2Config(
			WithSwithPortMode(SwitchPortModeTrunk),
			WithNativeVlan(200),
			WithAllowedVlans([]uint16{10, 20, 30}),
		)
		if err != nil {
			t.Fatalf("unexpected error while creating L2 config: %v", err)
		}

		if l2.nativeVlan == nil || *l2.nativeVlan != 200 {
			t.Errorf("expected nativeVlan to be 200, got %v", l2.nativeVlan)
		}
		expected := []uint16{10, 20, 30}
		if !reflect.DeepEqual(l2.allowedVlans, expected) {
			t.Errorf("expected allowedVlans to be %v, got %v", expected, l2.allowedVlans)
		}
	})
	t.Run("Valid trunk mode with invalid vlan configuration", func(t *testing.T) {
		_, err := NewL2Config(
			WithSwithPortMode(SwitchPortModeTrunk),
			WithNativeVlan(0),
			WithAllowedVlans([]uint16{0, 4095}),
		)
		if err == nil {
			t.Fatal("error not triggered for invalid VLANs")
		}
	})
	t.Run("Invalid config: use of access mode options in trunk mode", func(t *testing.T) {
		_, err := NewL2Config(
			WithSwithPortMode(SwitchPortModeTrunk),
			WithAccessVlan(100),
		)
		if err == nil {
			t.Fatalf("misconfig error not triggered wile using edge mode options in trunk mode")
		}
	})
}

func TestWithSpanningTree_InvalidMode(t *testing.T) {
	_, err := NewL2Config(WithSpanningTree(SpanningTreeMode(99)))
	if err == nil {
		t.Error("expected error for invalid spanning tree mode, got nil")
	}
}

func TestWithSwitchPortMode_InvalidMode(t *testing.T) {
	_, err := NewL2Config(WithSwithPortMode(SwitchPortMode(99)))
	if err == nil {
		t.Error("expected error for invalid switch port mode, got nil")
	}
}
