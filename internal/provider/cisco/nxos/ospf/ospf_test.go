// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package ospf

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestNewOSPF(t *testing.T) {
	t.Run("Empty name", func(t *testing.T) {
		_, err := NewOSPF("", "1.2.3.4")
		if err == nil {
			t.Error("expected error for empty OSPF name, got nil")
		}
	})
	t.Run("Invalid ID", func(t *testing.T) {
		_, err := NewOSPF("test", "not-an-ip")
		if err == nil {
			t.Error("expected error for invalid OSPF ID, got nil")
		}
	})
}

func TestWithAdminState(t *testing.T) {
	o, err := NewOSPF("test", "1.1.1.1", WithAdminState(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.adminSt != false {
		t.Errorf("expected adminSt false, got %v", o.adminSt)
	}
}

func TestWithDistance(t *testing.T) {
	t.Run("Valid Distance", func(t *testing.T) {
		o, err := NewOSPF("test", "1.1.1.1", WithDistance(77))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.distance == 0 || o.distance != 77 {
			t.Errorf("expected distance 77, got %v", o.distance)
		}
	})
	t.Run("Invalid Distance", func(t *testing.T) {
		_, err := NewOSPF("test", "1.1.1.1", WithDistance(0))
		if err == nil {
			t.Error("expected error for distance < 1, got nil")
		}
	})
}

func TestWithReferenceBandwidthMbps(t *testing.T) {
	t.Run("Valid ReferenceBandwidth", func(t *testing.T) {
		o, err := NewOSPF("test", "1.1.1.1", WithReferenceBandwidthMbps(1000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.referenceBandwidthMbps == 0 || o.referenceBandwidthMbps != 1000 {
			t.Errorf("expected referenceBandwidthMbps 1000, got %v", o.referenceBandwidthMbps)
		}
	})
	t.Run("Invalid ReferenceBandwidth", func(t *testing.T) {
		_, err := NewOSPF("test", "1.1.1.1", WithReferenceBandwidthMbps(0))
		if err == nil {
			t.Error("expected error for reference bandwidth < 1, got nil")
		}
		_, err = NewOSPF("test", "1.1.1.1", WithReferenceBandwidthMbps(1000000))
		if err == nil {
			t.Error("expected error for reference bandwidth > 999999, got nil")
		}
	})
}

func TestWithMaxLSA(t *testing.T) {
	t.Run("Valid MaxLSA", func(t *testing.T) {
		o, err := NewOSPF("test", "1.1.1.1", WithMaxLSA(12345))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.maxLSA == 0 || o.maxLSA != 12345 {
			t.Errorf("expected maxLSA 12345, got %v", o.maxLSA)
		}
	})
	t.Run("Invalid MaxLSA", func(t *testing.T) {
		_, err := NewOSPF("test", "1.1.1.1", WithMaxLSA(0))
		if err == nil {
			t.Error("expected error for maxLSA < 1, got nil")
		}
	})
}

func TestWithLogLevel(t *testing.T) {
	o, err := NewOSPF("test", "1.1.1.1", WithLogLevel(Brief))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.logLevel != Brief {
		t.Errorf("expected logLevel Brief, got %v", o.logLevel)
	}
	o, err = NewOSPF("test", "1.1.1.1", WithLogLevel(Detail))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.logLevel != Detail {
		t.Errorf("expected logLevel Detail, got %v", o.logLevel)
	}
	o, err = NewOSPF("test", "1.1.1.1", WithLogLevel(None))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.logLevel != None {
		t.Errorf("expected logLevel None, got %v", o.logLevel)
	}
	_, err = NewOSPF("test", "1.1.1.1", WithLogLevel(LogLevel(99)))
	if err == nil {
		t.Error("expected error for invalid log level, got nil")
	}
}

func TestWithDefaultRoutePropagation(t *testing.T) {
	o, err := NewOSPF("test", "1.1.1.1", WithDefaultRoutePropagation(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !o.progateDefaultRoute {
		t.Errorf("expected progateDefaultRoute true, got %v", o.progateDefaultRoute)
	}
}

func TestWithRedistributionConfig(t *testing.T) {
	t.Run("DistributionProtocolDirect", func(t *testing.T) {
		o, err := NewOSPF("test", "1.1.1.1", WithRedistributionConfig(DistributionProtocolDirect, "RM1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(o.redistributionConfigs) != 1 || o.redistributionConfigs[0].protocol != DistributionProtocolDirect {
			t.Errorf("expected redistributionConfigs with DistributionProtocolDirect, got %+v", o.redistributionConfigs)
		}
	})
	t.Run("DistributionProtocolStatic", func(t *testing.T) {
		o, err := NewOSPF("test", "1.1.1.1", WithRedistributionConfig(DistributionProtocolStatic, "RM2"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(o.redistributionConfigs) != 1 || o.redistributionConfigs[0].protocol != DistributionProtocolStatic {
			t.Errorf("expected redistributionConfigs with DistributionProtocolStatic, got %+v", o.redistributionConfigs)
		}
	})
	t.Run("Non implemented", func(t *testing.T) {
		nonImplemented := []DistributionProtocol{
			DistributionProtocol(99),
		}
		for _, p := range nonImplemented {
			_, err := NewOSPF("test", "1.1.1.1", WithRedistributionConfig(p, "RM3"))
			if err == nil {
				t.Error("expected error for unsupported redistribution protocol, got nil")
			}
		}
	})
	t.Run("DuplicateConfig", func(t *testing.T) {
		_, err := NewOSPF("test", "1.1.1.1", WithRedistributionConfig(DistributionProtocolDirect, "RM1"), WithRedistributionConfig(DistributionProtocolDirect, "RM1"))
		if err == nil {
			t.Fatalf("expected error with duplicate redistribution config, got nil")
		}
	})
	t.Run("EmptyRouteMapName", func(t *testing.T) {
		_, err := NewOSPF("test", "1.1.1.1", WithRedistributionConfig(DistributionProtocolDirect, ""))
		if err == nil {
			t.Fatalf("expected error for empty route map name, got nil")
		}
	})
}

func TestOSPF_ToYGOT(t *testing.T) {
	o, err := NewOSPF(
		"test",
		"1.1.1.1",
		WithAdminState(true),
		WithDistance(10),
		WithReferenceBandwidthMbps(1000),
		WithMaxLSA(100),
		WithLogLevel(Brief),
		WithDefaultRoutePropagation(true),
		WithRedistributionConfig(DistributionProtocolDirect, "RM1"),
		WithRedistributionConfig(DistributionProtocolStatic, "RM2"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := o.ToYGOT(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 update, got %d", len(updates))
	}
	t.Run("First update enables OSPF feature", func(t *testing.T) {
		u, ok := updates[0].(gnmiext.EditingUpdate)
		if !ok {
			t.Fatalf("expected EditingUpdate, got %T", updates[0])
		}
		if u.XPath != "System/fm-items/ospf-items" {
			t.Errorf("expected XPath 'System/fm-items/ospf-items', got %s", u.XPath)
		}
		val, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems)
		if !ok {
			t.Fatalf("expected value to be *OspfItems, got %T", u.Value)
		}
		if val.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
			t.Errorf("expected AdminSt enabled, got %v", val.AdminSt)
		}
	})
	t.Run("Second update configures OSPF process", func(t *testing.T) {
		ru, ok := updates[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Fatalf("expected ReplacingUpdate, got %T", updates[1])
		}
		if ru.XPath != "System/ospf-items/inst-items/Inst-list" {
			t.Errorf("expected XPath 'System/ospf-items/inst-items/Inst-list', got %s", ru.XPath)
		}
		val, ok := ru.Value.(*nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList)
		if !ok {
			t.Fatalf("expected value to be *InstList, got %T", ru.Value)
		}
		if *val.Name != "test" {
			t.Errorf("expected Name 'test', got %v", *val.Name)
		}
		if val.AdminSt != nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled {
			t.Errorf("expected AdminSt enabled, got %v", val.AdminSt)
		}
		if val.DomItems.DomList["default"].RtrId == nil || *val.DomItems.DomList["default"].RtrId != "1.1.1.1" {
			t.Errorf("expected RtrId 1.1.1.1, got %v", val.DomItems.DomList["default"].RtrId)
		}
	})
}

func TestOSPF_Reset(t *testing.T) {
	o, err := NewOSPF("test", "1.1.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := o.Reset(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	du, ok := updates[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Fatalf("expected DeletingUpdate, got %T", updates[0])
	}
	expectedXPath := "System/ospf-items/inst-items/Inst-list[name=test]"
	if du.XPath != expectedXPath {
		t.Errorf("expected XPath %q, got %q", expectedXPath, du.XPath)
	}
}
