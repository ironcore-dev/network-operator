// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package isis

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.wdf.sap.corp/cc/network-fabric-operator/pkg/gnmiext"
)

// test a configuration with only ISIS for IPv6
func TestToYGOT(t *testing.T) {
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
	if len(got) != 1 {
		t.Fatalf("ToYGOT() expected 1 update, got %d", len(got))
	}
	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}
	if update.XPath != "System/isis-items" {
		t.Errorf("expected XPath 'System/isis-items', got %s", update.XPath)
	}
	v, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_IsisItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_IsisItems")
	}
	instList := v.GetInstItems().GetInstList("UNDERLAY")
	if instList == nil {
		t.Fatalf("expected instList for UNDERLAY to be present")
	}
	domList := instList.GetDomItems().GetDomList("default")
	if domList == nil {
		t.Fatalf("expected domList for default to be present")
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
