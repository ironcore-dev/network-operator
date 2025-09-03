// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
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
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		// Valid names
		{"valid: Ethernet1/1", "Ethernet1/1", false},
		{"valid: eth1/1", "eth1/1", false},
		// Invalid names
		{"invalid: lo1", "lo1", true},
		{"invalid: empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPhysicalInterface(tt.input)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
		})
	}
}

func Test_PhysIf_ToYGOT_WithOptions_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		options     []PhysIfOption
		shouldError bool
	}{
		{
			name:        "valid: with description",
			options:     []PhysIfOption{WithDescription("test interface")},
			shouldError: false,
		},
		{
			name:        "invalid: nil L2 config",
			options:     []PhysIfOption{WithPhysIfL2(nil)},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPhysicalInterface("eth1/1", tt.options...)
			if (err != nil) != tt.shouldError {
				t.Fatalf("Expected error: %v, got error: %v", tt.shouldError, err)
			}
		})
	}
}

// mustNewL2Config is a helper to create L2Config and panic on error.
func mustNewL2Config(opts ...L2Option) *L2Config {
	l2cfg, err := NewL2Config(opts...)
	if err != nil {
		panic("failed to create L2Config: " + err.Error())
	}
	return l2cfg
}

// mustNewL2Config is a helper to create L3Config and panic on error.
func mustNewL3Config(opts ...L3Option) *L3Config {
	l3cfg, err := NewL3Config(opts...)
	if err != nil {
		panic("failed to create L3Config: " + err.Error())
	}
	return l3cfg
}

type updateCheck struct {
	updateIdx   int    // the position we want to check in the returned slice of updates
	expectType  string // "EditingUpdate" or "ReplacingUpdate"
	expectXPath string // the expected XPath of the update
	expectValue any    // the expected ygot object that should be in the update
}

func Test_PhysIf_ToYGOT_BaseConfig(t *testing.T) {
	tests := []struct {
		name                    string
		ifName                  string
		options                 []PhysIfOption
		expectedNumberOfUpdates int
		updateChecks            []updateCheck
	}{
		{
			name:                    "No additional base options",
			ifName:                  "eth1/1",
			options:                 []PhysIfOption{WithDescription("this is a test")},
			expectedNumberOfUpdates: 1,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("this is a test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						UserCfgdFlags: ygot.String("admin_state"),
					},
				},
			},
		},
		{
			name:   "MTU and VRF",
			ifName: "eth1/2",
			options: []PhysIfOption{
				WithDescription("this is a second test"),
				WithPhysIfMTU(9216),
				WithPhysIfVRF(physIfVRFName),
			},
			expectedNumberOfUpdates: 1,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/2]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("this is a second test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Mtu:           ygot.Uint32(9216),
						UserCfgdFlags: ygot.String("admin_mtu,admin_state"),
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
			},
		},
		{
			name:   "L2 then L3, expect only L3",
			ifName: "eth1/4",
			options: []PhysIfOption{
				WithDescription("L2 then L3 test"),
				WithPhysIfL2(mustNewL2Config(
					WithSpanningTree(SpanningTreeModeEdge),
					WithSwithPortMode(SwitchPortModeTrunk),
				)),
				WithPhysIfL3(mustNewL3Config(
					WithMedium(L3MediumTypeP2P),
					WithUnnumberedAddressing("loopback0"),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/4]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 then L3 test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
			},
		},
		{
			name:   "L3 then L2, expect only L2",
			ifName: "eth1/5",
			options: []PhysIfOption{
				WithDescription("L3 then L2 test"),
				WithPhysIfL3(mustNewL3Config(
					WithMedium(L3MediumTypeP2P),
					WithUnnumberedAddressing("loopback0"),
				)),
				WithPhysIfL2(mustNewL2Config(
					WithSpanningTree(SpanningTreeModeEdge),
					WithSwithPortMode(SwitchPortModeAccess),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/5]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L3 then L2 test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_access,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
			},
		},
		{
			name:   "L2 trunk configuration",
			ifName: "eth1/3",
			options: []PhysIfOption{
				WithDescription("L2 trunk test"),
				WithPhysIfL2(mustNewL2Config(
					WithSpanningTree(SpanningTreeModeEdge),
					WithSwithPortMode(SwitchPortModeTrunk),
					WithNativeVlan(100),
					WithAllowedVlans([]uint16{10, 20, 30}),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/3]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 trunk test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_trunk,
						NativeVlan:    ygot.String("vlan-100"),
						TrunkVlans:    ygot.String("10,20,30"),
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/3]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
						Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_edge,
					},
				},
			},
		},
		{
			name:   "L2 access configuration",
			ifName: "eth2/2",
			options: []PhysIfOption{
				WithDescription("L2 access test"),
				WithPhysIfL2(mustNewL2Config(
					WithSpanningTree(SpanningTreeModeEdge),
					WithSwithPortMode(SwitchPortModeAccess),
					WithAccessVlan(10),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth2/2]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 access test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_access,
						AccessVlan:    ygot.String("vlan-10"),
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/stp-items/inst-items/if-items/If-list[id=eth2/2]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
						Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_edge,
					},
				},
			},
		},
		{
			name:   "L3 unnumbered configuration",
			ifName: "eth1/1",
			options: []PhysIfOption{
				WithDescription("test interface"),
				WithPhysIfL3(mustNewL3Config(
					WithMedium(L3MediumTypeP2P),
					WithUnnumberedAddressing("loopback0"),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Descr:         ygot.String("test interface"),
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						Unnumbered: ygot.String("lo0"),
					},
				},
			},
		},
		{
			name:   "L3 numbered configuration",
			ifName: "eth3/1",
			options: []PhysIfOption{
				WithDescription("test interface"),
				WithPhysIfL3(mustNewL3Config(
					WithNumberedAddressingIPv4([]string{"192.0.2.1/8"}),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth3/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Descr:         ygot.String("test interface"),
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth3/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						AddrItems: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
							AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
								"192.0.2.1/8": {
									Addr: ygot.String("192.0.2.1/8"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "VRF with L3 unnumbered configuration",
			ifName: "eth1/1",
			options: []PhysIfOption{
				WithDescription("test interface"),
				WithPhysIfVRF(physIfVRFName),
				WithPhysIfL3(mustNewL3Config(
					WithMedium(L3MediumTypeP2P),
					WithUnnumberedAddressing("loopback0"),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("test interface"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id=eth1/1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						Unnumbered: ygot.String("lo0"),
					},
				},
			},
		},
		{
			name:   "VRF with L3 numbered configuration",
			ifName: "eth3/3",
			options: []PhysIfOption{
				WithDescription("test interface"),
				WithPhysIfVRF(physIfVRFName),
				WithPhysIfL3(mustNewL3Config(
					WithNumberedAddressingIPv4([]string{"192.0.2.1/8"}),
				)),
			},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/phys-items/PhysIf-list[id=eth3/3]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("test interface"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id=eth3/3]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						AddrItems: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
							AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
								"192.0.2.1/8": {
									Addr: ygot.String("192.0.2.1/8"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPhysicalInterface(tt.ifName, tt.options...)
			if err != nil {
				t.Fatalf("failed to create physical interface: %v", err)
			}

			updates, err := p.ToYGOT(t.Context(), &gnmiext.ClientMock{})
			if err != nil {
				t.Fatalf("unexpected error during ToYGOT: %v", err)
			}

			if len(updates) != tt.expectedNumberOfUpdates {
				t.Fatalf("expected %d updates, got %d", tt.expectedNumberOfUpdates, len(updates))
			}

			validateUpdates(t, updates, tt.updateChecks)
		})
	}
}

func Test_PhysIf_Reset(t *testing.T) {
	tests := []struct {
		name          string
		ifName        string
		options       []PhysIfOption
		expectUpdates []struct {
			XPath string
			Value any
		}
	}{
		{
			name:   "basic reset",
			ifName: "eth1/1",
			options: []PhysIfOption{
				WithDescription("test interface"),
			},
			expectUpdates: []struct {
				XPath string
				Value any
			}{
				{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
				{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
				},
			},
		},
		{
			name:   "reset with L2 configuration",
			ifName: "eth1/2",
			options: []PhysIfOption{
				WithDescription("L2 test interface"),
				WithPhysIfL2(&L2Config{}),
			},
			expectUpdates: []struct {
				XPath string
				Value any
			}{
				{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
				{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
				},
			},
		},
		{
			name:   "reset with L3 configuration",
			ifName: "eth1/3",
			options: []PhysIfOption{
				WithDescription("L3 test interface"),
				WithPhysIfL3(&L3Config{
					medium:             L3MediumTypeP2P,
					unnumberedLoopback: "lo0",
				}),
			},
			expectUpdates: []struct {
				XPath string
				Value any
			}{
				{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
				{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPhysicalInterface(tt.ifName, tt.options...)
			if err != nil {
				t.Fatalf("failed to create physical interface: %v", err)
			}

			got, err := p.Reset(context.Background(), nil)
			if err != nil {
				t.Errorf("unexpected error during reset: %v", err)
			}

			if len(got) != len(tt.expectUpdates) {
				t.Errorf("expected %d updates, got %d", len(tt.expectUpdates), len(got))
			}

			for i, expect := range tt.expectUpdates {
				if i >= len(got) {
					t.Errorf("missing update for expected xpath '%s'", expect.XPath)
					continue
				}
				update, ok := got[i].(gnmiext.ReplacingUpdate)
				if !ok {
					t.Errorf("expected value to be of type ReplacingUpdate at index %d", i)
					continue
				}
				if update.XPath != expect.XPath {
					t.Errorf("wrong xpath at index %d, expected '%s', got '%s'", i, expect.XPath, update.XPath)
				}

				expectValue := expect.Value.(ygot.GoStruct)
				notification, err := ygot.Diff(update.Value, expectValue)
				if err != nil {
					t.Errorf("failed to compute diff at index %d: %v", i, err)
				}
				if len(notification.Update) > 0 || len(notification.Delete) > 0 {
					t.Errorf("unexpected diff at index %d: %s", i, notification)
				}
			}
		})
	}
}

func validateUpdates(t *testing.T, updates []gnmiext.Update, updateChecks []updateCheck) {
	for _, check := range updateChecks {
		var update any
		switch check.expectType {
		case "EditingUpdate":
			update, _ = updates[check.updateIdx].(gnmiext.EditingUpdate)
		case "ReplacingUpdate":
			update, _ = updates[check.updateIdx].(gnmiext.ReplacingUpdate)
		default:
			t.Fatalf("unknown expectType: %s", check.expectType)
		}
		if update == nil {
			t.Errorf("expected value to be of type %s at index %d", check.expectType, check.updateIdx)
			continue
		}
		var xpath string
		var value any
		switch u := update.(type) {
		case gnmiext.EditingUpdate:
			xpath = u.XPath
			value = u.Value
		case gnmiext.ReplacingUpdate:
			xpath = u.XPath
			value = u.Value
		}
		if xpath != check.expectXPath {
			t.Errorf("wrong xpath at index %d, expected '%s', got '%s'", check.updateIdx, check.expectXPath, xpath)
		}
		valueGoStruct, ok1 := value.(ygot.GoStruct)
		expectValueGoStruct, ok2 := check.expectValue.(ygot.GoStruct)
		if !ok1 || !ok2 {
			t.Errorf("failed to type assert value or expectValue to ygot.GoStruct at index %d", check.updateIdx)
			continue
		}
		notification, err := ygot.Diff(valueGoStruct, expectValueGoStruct)
		if err != nil {
			t.Errorf("failed to compute diff: %v", err)
		}
		if len(notification.Update) > 0 || len(notification.Delete) > 0 {
			t.Errorf("unexpected diff at index %d: %s", check.updateIdx, notification)
		}
	}
}
