// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iface

import (
	"context"
	"errors"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

const (
	portChannelName        = "po10"
	portChannelDescription = "test port-channel"
	physIf1                = "eth1/1"
	physIf2                = "eth1/2"
)

func Test_PortChannel_NewPortChannel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		// Valid names
		{"valid: Port-Channel10", "Port-Channel10", false},
		{"valid: port-channel10", "port-channel10", false},
		{"valid: po10", "po10", false},
		// Invalid names
		{"invalid: lo1", "lo1", true},
		{"invalid: poo1", "poo1", true},
		{"invalid: eth1/1", "eth1/1", true},
		// Boundary cases
		{"invalid: empty string", "", true},
		{"invalid: port-channel0", "port-channel0", true},
		{"invalid: port-channel4097", "port-channel4097", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPortChannel(tt.input)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
		})
	}
}

func Test_PortChannel_ToYGOT_WithOptions_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		options     []PortChannelOption
		shouldError bool
	}{
		{
			name:        "valid: single physical interface",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1")},
			shouldError: false,
		},
		{
			name:        "invalid: loopback as physical interface",
			options:     []PortChannelOption{WithPhysicalInterface("lo1")},
			shouldError: true,
		},
		{
			name:        "invalid: port-channel as physical interface",
			options:     []PortChannelOption{WithPhysicalInterface("po1")},
			shouldError: true,
		},
		{
			name:        "invalid: a list with valid and invalid interfaces",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1"), WithPhysicalInterface("po1")},
			shouldError: true,
		},
		{
			name:        "valid: multiple valid physical interfaces",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1"), WithPhysicalInterface("eth1/2")},
			shouldError: false,
		},
		{
			name:        "invalid: empty description",
			options:     []PortChannelOption{WithPortChannelDescription("")},
			shouldError: true,
		},
		{
			name:        "invalid: nil L2 config",
			options:     []PortChannelOption{WithPortChannelL2(nil)},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPortChannel("po1", tt.options...)
			if (err != nil) != tt.shouldError {
				t.Fatalf("Expected error: %v, got error: %v", tt.shouldError, err)
			}
		})
	}
}

// Test_PortChannel_ToYGOT_GnmiClient tests interactions with the gnmi client
func Test_PortChannel_ToYGOT_GnmiClient(t *testing.T) {
	tests := []struct {
		name        string
		opts        []PortChannelOption
		client      *gnmiext.ClientMock
		expectError bool
	}{
		{
			name: "invalid physical interface",
			opts: []PortChannelOption{WithPhysicalInterface("eth1/1")},
			client: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) {
					return true, nil
				},
			},
			expectError: false,
		},
		{
			name: "physical interface does not exist",
			opts: []PortChannelOption{WithPhysicalInterface("eth1/1")},
			client: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) {
					return false, nil
				},
			},
			expectError: true,
		},
		{
			name: "client error while checking if physical interface exists",
			opts: []PortChannelOption{WithPhysicalInterface("eth1/1")},
			client: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) {
					return false, errors.New("error")
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPortChannel(portChannelName, tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error during NewPortChannel: %v", err)
			}
			_, err = p.ToYGOT(t.Context(), tt.client)
			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectError, err)
			}
		})
	}
}

func Test_PortChannel_ToYGOT_Updates(t *testing.T) {
	type updateCheck struct {
		updateIdx   int    // the position we want to check in the returned slice of updates
		expectType  string // "EditingUpdate" or "ReplacingUpdate"
		expectXPath string // the expected XPath of the update
		expectValue any    // the expected ygot object that should be in the update
	}

	tests := []struct {
		name                    string
		pcName                  string
		options                 []PortChannelOption
		expectedNumberOfUpdates int
		updateChecks            []updateCheck
	}{
		{
			name:                    "1st update is LACP feature",
			pcName:                  "po1",
			options:                 []PortChannelOption{},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "EditingUpdate",
					expectXPath: "System/fm-items/lacp-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_LacpItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
			},
		},
		{
			name:                    "vanilla port-channel config",
			pcName:                  "po1",
			options:                 []PortChannelOption{},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/aggr-items/AggrIf-list[id=po1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						UserCfgdFlags: ygot.String("admin_state"),
						PcMode:        nxos.Cisco_NX_OSDevice_Pc_Mode_active,
					},
				},
			},
		},
		{
			name:   "configured as with trunk with vlan",
			pcName: "po1",
			options: []PortChannelOption{
				WithPortChannelDescription("trunk port-channel"),
				WithPhysicalInterface("eth1/1"),
				WithPhysicalInterface("eth1/2"),
				WithPortChannelL2(func() *L2Config {
					l2cfg, err := NewL2Config(
						WithSpanningTree(SpanningTreeModeNetwork),
						WithSwithPortMode(SwitchPortModeTrunk),
						WithAllowedVlans([]uint16{10, 20}),
						WithNativeVlan(200),
					)
					if err != nil {
						t.Fatalf("unexpected error while creating L2 config: %v", err)
					}
					return l2cfg
				}()),
			},
			expectedNumberOfUpdates: 3,
			updateChecks: []updateCheck{
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/intf-items/aggr-items/AggrIf-list[id=po1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Descr:         ygot.String("trunk port-channel"),
						PcMode:        nxos.Cisco_NX_OSDevice_Pc_Mode_active,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_trunk,
						NativeVlan:    ygot.String("vlan-200"),
						TrunkVlans:    ygot.String("10,20"),
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
						RsmbrIfsItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList_RsmbrIfsItems{
							RsMbrIfsList: map[string]*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList_RsmbrIfsItems_RsMbrIfsList{
								"System/intf-items/phys-items/PhysIf-list[id=eth1/1]": {
									TDn: ygot.String("System/intf-items/phys-items/PhysIf-list[id=eth1/1]"),
								},
								"System/intf-items/phys-items/PhysIf-list[id=eth1/2]": {
									TDn: ygot.String("System/intf-items/phys-items/PhysIf-list[id=eth1/2]"),
								},
							},
						},
					},
				},
				{
					updateIdx:   2,
					expectType:  "ReplacingUpdate",
					expectXPath: "System/stp-items/inst-items/if-items/If-list[id=po1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
						Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_network,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := NewPortChannel(tt.pcName, tt.options...)
			if err != nil {
				t.Fatalf("unexpected error during NewPortChannel: %v", err)
			}
			updates, err := pc.ToYGOT(t.Context(), &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return true, nil },
			})
			if err != nil {
				t.Fatalf("unexpected error during ToYGOT: %v", err)
			}
			if len(updates) != tt.expectedNumberOfUpdates {
				t.Fatalf("expected %d updates, got %d", tt.expectedNumberOfUpdates, len(updates))
			}
			for _, check := range tt.updateChecks {
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
		})
	}
}

func Test_PortChannel_Reset(t *testing.T) {
	tests := []struct {
		name         string
		pcName       string
		options      []PortChannelOption
		expectXPaths []string
	}{
		{
			name:    "basic reset: also enforce removal of in stp path",
			pcName:  "po10",
			options: nil,
			expectXPaths: []string{
				"System/intf-items/aggr-items/AggrIf-list[id=po10]",
				"System/stp-items/inst-items/if-items/If-list[id=po10]",
			},
		},
		{
			name:   "reset with physical interfaces",
			pcName: "po99",
			options: []PortChannelOption{
				WithPhysicalInterface("eth1/1"),
				WithPhysicalInterface("eth1/2"),
				WithPortChannelL2(func() *L2Config {
					l2cfg, err := NewL2Config(
						WithSpanningTree(SpanningTreeModeNetwork),
					)
					if err != nil {
						t.Fatalf("unexpected error while creating L2 config: %v", err)
					}
					return l2cfg
				}()),
			},
			expectXPaths: []string{
				"System/intf-items/aggr-items/AggrIf-list[id=po99]",
				"System/stp-items/inst-items/if-items/If-list[id=po99]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPortChannel(tt.pcName, tt.options...)
			if err != nil {
				t.Fatalf("failed to create port-channel: %v", err)
			}
			got, err := p.Reset(t.Context(), nil)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(got) != len(tt.expectXPaths) {
				t.Errorf("expected %d updates, got %d", len(tt.expectXPaths), len(got))
			}
			for i, expectXPath := range tt.expectXPaths {
				if i >= len(got) {
					t.Errorf("missing update for expected xpath '%s'", expectXPath)
				}
				del, ok := got[i].(gnmiext.DeletingUpdate)
				if !ok {
					t.Errorf("expected value to be of type DeletingUpdate at index %d", i)
				}
				if del.XPath != expectXPath {
					t.Errorf("wrong xpath at index %d, expected '%s', got '%s'", i, expectXPath, del.XPath)
				}
			}
		})
	}
}
