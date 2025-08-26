// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package isis

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

// TestToYGOT tests a configuration with only ISIS for IPv6
func Test_ISIS_ToYGOT(t *testing.T) {
	isis := &ISIS{
		Name:  "UNDERLAY",
		NET:   "49.0001.0001.0000.0001.00",
		Level: Level12,
		OverloadBit: &OverloadBit{
			OnStartup: 61, // seconds
		},
		AddressFamilies: []ISISAFType{
			IPv6Unicast,
		},
	}
	got, err := isis.ToYGOT(nil)
	if err != nil {
		t.Fatalf("ToYGOT() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ToYGOT() expected 2 updates, got %d", len(got))
	}
	edit, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected first update to be of type ReplacingUpdate")
	}
	if edit.XPath != "System/fm-items/isis-items" {
		t.Errorf("expected first update XPath 'System/fm-items/isis-items', got %s", edit.XPath)
	}
	update, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected second update to be of type ReplacingUpdate")
	}
	if update.XPath != "System/isis-items/inst-items/Inst-list[name=UNDERLAY]" {
		t.Errorf("expected XPath 'System/isis-items/inst-items/Inst-list[name=UNDERLAY]', got %s", update.XPath)
	}
	instList, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList")
	}
	if instList.Name == nil || *instList.Name != "UNDERLAY" {
		t.Errorf("expected instList.Name to be 'UNDERLAY', got %v", instList.Name)
	}
	domItems := instList.GetDomItems()
	if domItems == nil {
		t.Fatalf("expected DomItems to be present")
	}
	domList := domItems.GetDomList("default")
	if domList == nil {
		t.Fatalf("expected domList for default to be present")
	}
	if domList.Net == nil {
		t.Fatalf("expected Net to be set")
	}
	if *domList.Net != isis.NET {
		t.Errorf("Net not set correctly")
	}
	if domList.IsType != nxos.Cisco_NX_OSDevice_Isis_IsT_l12 {
		t.Errorf("Level not set correctly")
	}
	if domList.GetOverloadItems().AdminSt != nxos.Cisco_NX_OSDevice_Isis_OverloadAdminSt_bootup {
		t.Errorf("OverloadBit AdminSt not set correctly")
	}
	if *domList.GetOverloadItems().StartupTime != isis.OverloadBit.OnStartup {
		t.Errorf("OverloadBit StartupTime not set correctly")
	}
	if len(domList.GetAfItems().DomAfList) != 1 {
		t.Errorf("expected 1 address family")
	}
	if domList.GetAfItems().GetDomAfList(nxos.Cisco_NX_OSDevice_Isis_AfT_v6) == nil {
		t.Errorf("expected IPv6 unicast to be enabled, but it is disabled")
	}
}

func Test_ISIS_ToYGOT_InvalidLevel(t *testing.T) {
	isis := &ISIS{
		Name:            "UNDERLAY",
		NET:             "49.0001.0001.0000.0001.00",
		Level:           ISISType(99),
		AddressFamilies: []ISISAFType{IPv4Unicast},
	}
	_, err := isis.ToYGOT(&gnmiext.ClientMock{})
	if err == nil {
		t.Error("expected error for invalid level, got nil")
	}
}

func Test_ISIS_ToYGOT_InvalidAddressFamily(t *testing.T) {
	isis := &ISIS{
		Name:            "UNDERLAY",
		NET:             "49.0001.0001.0000.0001.00",
		Level:           Level1,
		AddressFamilies: []ISISAFType{ISISAFType(99)},
	}
	_, err := isis.ToYGOT(&gnmiext.ClientMock{})
	if err == nil {
		t.Error("expected error for invalid address family, got nil")
	}
}

func Test_ISIS_ToYGOT_NoOverloadBit(t *testing.T) {
	isis := &ISIS{
		Name:            "UNDERLAY",
		NET:             "49.0001.0001.0000.0001.00",
		Level:           Level1,
		AddressFamilies: []ISISAFType{IPv4Unicast},
		OverloadBit:     nil,
	}
	_, err := isis.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error when OverloadBit is nil: %v", err)
	}
}

func Test_ISIS_ToYGOT_EmptyAddressFamilies(t *testing.T) {
	isis := &ISIS{
		Name:            "UNDERLAY",
		NET:             "49.0001.0001.0000.0001.00",
		Level:           Level1,
		AddressFamilies: []ISISAFType{},
	}
	updates, err := isis.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error for empty address families: %v", err)
	}
	if len(updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(updates))
	}
}

func Test_ISIS_Reset(t *testing.T) {
	isis := &ISIS{Name: "UNDERLAY"}
	updates, err := isis.Reset(&gnmiext.ClientMock{})
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
	expectedXPath := "System/isis-items/inst-items/Inst-list[name=UNDERLAY]"
	if du.XPath != expectedXPath {
		t.Errorf("expected XPath %q, got %q", expectedXPath, du.XPath)
	}
}

func Test_ISIS_MissingMandatoryFields(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		isis := &ISIS{
			NET:             "49.0001.0001.0000.0001.00",
			Level:           Level1,
			AddressFamilies: []ISISAFType{IPv4Unicast},
		}
		_, err := isis.ToYGOT(&gnmiext.ClientMock{})
		if err == nil {
			t.Error("expected error for empty name, got nil")
		}
	})
	t.Run("missing NET", func(t *testing.T) {
		isis := &ISIS{
			Name:            "UNDERLAY",
			Level:           Level1,
			AddressFamilies: []ISISAFType{IPv4Unicast},
		}
		_, err := isis.ToYGOT(&gnmiext.ClientMock{})
		if err == nil {
			t.Error("expected error for empty NET, got nil")
		}
	})
}
