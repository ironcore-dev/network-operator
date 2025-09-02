// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package isis

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Interface_NewInterface(t *testing.T) {
	tests := []struct {
		name      string
		ifName    string
		isisInst  string
		opts      []IfOption
		wantShort string
		wantVRF   string
		wantV4    bool
		wantV6    bool
		wantP2P   bool
		expectErr bool
	}{
		{
			name:      "Physical, default VRF, default AFs",
			ifName:    "Ethernet1/1",
			isisInst:  "ISIS1",
			wantShort: "eth1/1",
			wantVRF:   "default",
			wantV4:    true,
			wantV6:    true,
			wantP2P:   false,
		},
		{
			name:      "Loopback, custom VRF, IPv4 only, P2P",
			ifName:    "lo0",
			isisInst:  "ISIS2",
			opts:      []IfOption{WithVRF("mgmt"), WithIPv6(false), WithPointToPoint()},
			wantShort: "lo0",
			wantVRF:   "mgmt",
			wantV4:    true,
			wantV6:    false,
			wantP2P:   true,
		},
		{
			name:      "Physical, IPv6 only",
			ifName:    "Ethernet1/2",
			isisInst:  "ISIS3",
			opts:      []IfOption{WithIPv4(false)},
			wantShort: "eth1/2",
			wantVRF:   "default",
			wantV4:    false,
			wantV6:    true,
			wantP2P:   false,
		},
		{
			name:      "Invalid interface name",
			ifName:    "notanif",
			isisInst:  "ISIS4",
			expectErr: true,
		},
		{
			name:      "Empty ISIS instance name",
			ifName:    "Ethernet1/3",
			isisInst:  "",
			expectErr: true,
		},
		{
			name:      "Empty VRF",
			ifName:    "Ethernet1/4",
			isisInst:  "ISIS5",
			opts:      []IfOption{WithVRF("")},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			intf, err := NewInterface(tc.ifName, tc.isisInst, tc.opts...)
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
			if intf.v4Enable != tc.wantV4 {
				t.Errorf("expected v4Enable %v, got %v", tc.wantV4, intf.v4Enable)
			}
			if intf.v6Enable != tc.wantV6 {
				t.Errorf("expected v6Enable %v, got %v", tc.wantV6, intf.v6Enable)
			}
		})
	}
}

func Test_Interface_ToYGOT(t *testing.T) {
	i, err := NewInterface("Ethernet1/1", "ISIS1",
		WithVRF("mgmt"),
		WithIPv4(true),
		WithIPv6(false),
		WithPointToPoint(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := i.ToYGOT(t.Context(), &gnmiext.ClientMock{
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
	t.Run("Enable ISIS feature", func(t *testing.T) {
		u, ok := updates[0].(gnmiext.EditingUpdate)
		if !ok {
			t.Fatalf("expected EditingUpdate, got %T", updates[0])
		}
		if u.XPath != "System/fm-items/isis-items" {
			t.Errorf("expected XPath System/fm-items/isis-items, got %s", u.XPath)
		}
		gotVal, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_IsisItems)
		if !ok {
			t.Errorf("expected value of type *nxos.Cisco_NX_OSDevice_System_FmItems_IsisItems, got %T", u.Value)
		}
		if gotVal.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
			t.Errorf("expected AdminSt enabled, got %v", gotVal.AdminSt)
		}
	})
	t.Run("Configure ISIS interface", func(t *testing.T) {
		u, ok := updates[1].(gnmiext.ReplacingUpdate)
		if !ok {
			t.Fatalf("expected ReplacingUpdate, got %T", updates[1])
		}
		if u.XPath != "System/isis-items/if-items/InternalIf-list[id="+i.interfaceName+"]" {
			t.Errorf("unexpected XPath, got %s", u.XPath)
		}
		gotVal, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList)
		if !ok {
			t.Errorf("expected value of type *nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList, got %T", u.Value)
		}
		expectedVal := &nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList{
			Dom:            ygot.String(i.vrf),
			Instance:       ygot.String(i.instanceName),
			V4Enable:       ygot.Bool(i.v4Enable),
			V6Enable:       ygot.Bool(i.v6Enable),
			NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_on,
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

func Test_Interface_ToYGOT_NoP2P(t *testing.T) {
	i, err := NewInterface("Ethernet1/1", "ISIS1",
		WithVRF("mgmt"),
		WithIPv4(true),
		WithIPv6(false),
		// No WithPointToPoint()
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := i.ToYGOT(t.Context(), &gnmiext.ClientMock{
		ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from ToYGOT: %v", err)
	}
	u, ok := updates[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Fatalf("expected ReplacingUpdate, got %T", updates[1])
	}
	gotVal, ok := u.Value.(*nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList)
	if !ok {
		t.Fatalf("expected value of type *nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList, got %T", u.Value)
	}
	// Check that NetworkTypeP2P is not set or is set to the default
	if gotVal.NetworkTypeP2P != nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_UNSET {
		t.Errorf("expected NetworkTypeP2P to be unset or off, got %v", gotVal.NetworkTypeP2P)
	}
}

func TestReset(t *testing.T) {
	i, err := NewInterface("Ethernet1/1", "ISIS1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := i.Reset(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error from Reset: %v", err)
	}
	if len(updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(updates))
	}
	u, ok := updates[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Fatalf("expected DeletingUpdate, got %T", updates[0])
	}
	if u.XPath != "System/isis-items/if-items/InternalIf-list[id="+i.interfaceName+"]" {
		t.Errorf("unexpected XPath, got %s", u.XPath)
	}
}
