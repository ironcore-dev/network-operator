// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package ospf

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Interface_NewInterface(t *testing.T) {
	tests := []struct {
		name        string
		ifName      string
		ospfInst    string
		area        string
		opts        []IfOption
		wantShort   string
		wantVRF     string
		wantIsP2P   bool
		wantPassive bool
		expectErr   bool
	}{
		{
			name:      "Physical, default VRF, default network type (name variation 1)",
			ifName:    "Ethernet1/1",
			ospfInst:  "OSPF1",
			area:      "0",
			wantShort: "eth1/1",
			wantVRF:   "default",
		},
		{
			name:      "Physical, default VRF, default network type (name variation 2)",
			ifName:    "eth1/1",
			ospfInst:  "OSPF1",
			area:      "0",
			wantShort: "eth1/1",
			wantVRF:   "default",
		},
		{
			name:      "Physical, default VRF, default network type (name variation 3)",
			ifName:    "ethernet1/1",
			ospfInst:  "OSPF1",
			area:      "0",
			wantShort: "eth1/1",
			wantVRF:   "default",
		},
		{
			name:      "Loopback, default VRF, default network type (name variation 1)",
			ifName:    "Loopback0",
			ospfInst:  "OSPF1",
			area:      "0",
			wantShort: "lo0",
			wantVRF:   "default",
		},
		{
			name:      "Physical, default VRF, default network type (name variation 2)",
			ifName:    "lo0",
			ospfInst:  "OSPF1",
			area:      "0",
			wantShort: "lo0",
			wantVRF:   "default",
		},
		{
			name:      "Loopback, custom VRF",
			ifName:    "lo0",
			ospfInst:  "OSPF2",
			area:      "0.0.0.1",
			opts:      []IfOption{WithVRF("mgmt")},
			wantShort: "lo0",
			wantVRF:   "mgmt",
		},
		{
			name:        "Physical, P2P, passive mode disabled",
			ifName:      "Ethernet1/2",
			ospfInst:    "OSPF3",
			area:        "1",
			opts:        []IfOption{WithP2PNetworkType(), WithDisablePassiveMode()},
			wantShort:   "eth1/2",
			wantVRF:     "default",
			wantIsP2P:   true,
			wantPassive: true,
		},
		{
			name:      "empty interface name",
			ifName:    "",
			ospfInst:  "OSPF4",
			area:      "0.0.0.0",
			expectErr: true,
		},
		{
			name:      "Physical, invalid are (1)",
			ifName:    "Ethernet1/3",
			ospfInst:  "OSPF4",
			area:      "random",
			expectErr: true,
		},
		{
			name:      "Physical, invalid area (2)",
			ifName:    "Ethernet1/3",
			ospfInst:  "OSPF4",
			area:      "256.0.0.1",
			expectErr: true,
		},
		{
			name:      "Physical, invalid area (4)",
			ifName:    "Ethernet1/3",
			ospfInst:  "OSPF4",
			area:      "4294967296",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			intf, err := NewInterface(tc.ifName, tc.ospfInst, tc.area, tc.opts...)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if intf.interfaceName != tc.wantShort {
				t.Errorf("expected short interface name %q, got %q", tc.wantShort, intf.interfaceName)
			}
			if intf.vrf != tc.wantVRF {
				t.Errorf("expected VRF %q, got %q", tc.wantVRF, intf.vrf)
			}
			if intf.isP2P != tc.wantIsP2P {
				t.Errorf("expected isP2P %v, got %v", tc.wantIsP2P, intf.isP2P)
			}
			if intf.disablePassiveMode != tc.wantPassive {
				t.Errorf("expected disablePassiveMode %v, got %v", tc.wantPassive, intf.disablePassiveMode)
			}
		})
	}
}

func Test_Interface_ToYGOT(t *testing.T) {
	intf, err := NewInterface("Ethernet1/1", "OSPF1", "0",
		WithVRF("mgmt"),
		WithP2PNetworkType(),
		WithDisablePassiveMode(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := intf.ToYGOT(t.Context(), &gnmiext.ClientMock{
		ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from ToYGOT: %v", err)
	}
	if len(updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(updates))
	}
	t.Run("Enable OSPF feature", func(t *testing.T) {
		u, ok := updates[0].(gnmiext.EditingUpdate)
		if !ok {
			t.Fatalf("expected EditingUpdate, got %T", updates[0])
		}
		if u.XPath != "System/fm-items/ospf-items" {
			t.Errorf("expected XPath System/features/ospf, got %s", u.XPath)
		}
		gotVal, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems)
		if !ok {
			t.Errorf("expected value of type *nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems, got %T", u.Value)
		}
		expectedVal := &nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems{
			AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
		}
		diffNotifications, err := ygot.Diff(gotVal, expectedVal)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(diffNotifications.Update) > 0 || len(diffNotifications.Delete) > 0 {
			t.Errorf("unexpected diff: %s", diffNotifications)
		}
	})
	t.Run("Configure OSPF interface", func(t *testing.T) {
		u, ok := updates[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Fatalf("expected ReplacingUpdate, got %T", updates[1])
		}
		if u.XPath != "System/ospf-items/inst-items/Inst-list[name=OSPF1]/dom-items/Dom-list[name=mgmt]/if-items/If-list[id=eth1/1]" {
			t.Errorf("unexpected XPath, got %s", u.XPath)
		}
		gotVal, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_IfItems_IfList)
		if !ok {
			t.Errorf("expected value of type *nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_IfItems_IfList, got %T", u.Value)
		}
		expectedVal := &nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_IfItems_IfList{
			Area:                 ygot.String("0"),
			NwT:                  nxos.Cisco_NX_OSDevice_Ospf_NwT_p2p,
			AdvertiseSecondaries: ygot.Bool(true),
			PassiveCtrl:          nxos.Cisco_NX_OSDevice_Ospf_PassiveControl_disabled,
		}
		diffNotifications, err := ygot.Diff(gotVal, expectedVal)
		if err != nil {
			t.Errorf("failed to compute diff")
		}
		if len(diffNotifications.Update) > 0 || len(diffNotifications.Delete) > 0 {
			t.Errorf("unexpected diff: %s", diffNotifications)
		}
	})
}
