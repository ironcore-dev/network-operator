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
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
)

func Test_NewPortChannel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		options     []PortChannelOption
		shouldError bool
	}{
		// Valid names
		{
			name:        "valid: Port-Channel10",
			input:       "Port-Channel10",
			options:     nil,
			shouldError: false,
		},
		{
			name:        "valid: port-channel10",
			input:       "port-channel10",
			options:     nil,
			shouldError: false,
		},
		{
			name:        "valid: po10",
			input:       "po10",
			options:     nil,
			shouldError: false,
		},
		// Invalid names
		{
			name:        "invalid: lo1",
			input:       "lo1",
			options:     nil,
			shouldError: true,
		},
		{
			name:        "invalid: poo1",
			input:       "poo1",
			options:     nil,
			shouldError: true,
		},
		{
			name:        "invalid: eth1/1",
			input:       "eth1/1",
			options:     nil,
			shouldError: true,
		},
		// Boundary cases
		{
			name:        "invalid: empty string",
			input:       "",
			options:     nil,
			shouldError: true,
		},
		{
			name:        "invalid: port-channel0",
			input:       "port-channel0",
			options:     nil,
			shouldError: true,
		},
		{
			name:        "invalid: port-channel4097",
			input:       "port-channel4097",
			options:     nil,
			shouldError: true,
		},
		// Option validation
		{
			name:        "valid: single physical interface",
			input:       "po1",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1")},
			shouldError: false,
		},
		{
			name:        "invalid: loopback as physical interface",
			input:       "po1",
			options:     []PortChannelOption{WithPhysicalInterface("lo1")},
			shouldError: true,
		},
		{
			name:        "invalid: port-channel as physical interface",
			input:       "po1",
			options:     []PortChannelOption{WithPhysicalInterface("po1")},
			shouldError: true,
		},
		{
			name:        "invalid: a list with valid and invalid interfaces",
			input:       "po1",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1"), WithPhysicalInterface("po1")},
			shouldError: true,
		},
		{
			name:        "valid: multiple valid physical interfaces",
			input:       "po1",
			options:     []PortChannelOption{WithPhysicalInterface("eth1/1"), WithPhysicalInterface("eth1/2")},
			shouldError: false,
		},
		{
			name:        "invalid: empty description",
			input:       "po1",
			options:     []PortChannelOption{WithPortChannelDescription("")},
			shouldError: true,
		},
		{
			name:        "invalid: nil L2 config",
			input:       "po1",
			options:     []PortChannelOption{WithPortChannelL2(nil)},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPortChannel(tt.input, tt.options...)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
		})
	}
}

func Test_PortChannel_ToYGOT(t *testing.T) {
	tests := []struct {
		name            string
		pcName          string
		options         []PortChannelOption
		expectedUpdates []gnmiext.Update
		clientMock      *gnmiext.ClientMock
		expectErr       bool
	}{
		{
			name:    "1st update is LACP feature",
			pcName:  "po1",
			options: []PortChannelOption{},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/lacp-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_LacpItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/aggr-items/AggrIf-list[id=po1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						UserCfgdFlags: ygot.String("admin_state"),
						PcMode:        nxos.Cisco_NX_OSDevice_Pc_Mode_active,
					},
				},
			},
			clientMock: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return true, nil },
			},
			expectErr: false,
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/lacp-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_LacpItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/aggr-items/AggrIf-list[id=po1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{
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
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=po1]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
						Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_network,
					},
				},
			},
			clientMock: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return true, nil },
			},
			expectErr: false,
		},
		{
			name:    "physical interface does not exist",
			pcName:  "po1",
			options: []PortChannelOption{WithPhysicalInterface("eth1/1")},
			clientMock: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return false, nil },
			},
			expectErr: true,
		},
		{
			name:    "client error while checking if physical interface exists",
			pcName:  "po1",
			options: []PortChannelOption{WithPhysicalInterface("eth1/1")},
			clientMock: &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return false, errors.New("error") },
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := NewPortChannel(tt.pcName, tt.options...)
			if err != nil {
				if !tt.expectErr {
					t.Fatalf("unexpected error during NewPortChannel: %v", err)
				}
				return
			}
			updates, err := pc.ToYGOT(t.Context(), tt.clientMock)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
func Test_PortChannel_Reset(t *testing.T) {
	tests := []struct {
		name            string
		pcName          string
		options         []PortChannelOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:    "basic reset: also enforce removal of in stp path",
			pcName:  "po10",
			options: nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/intf-items/aggr-items/AggrIf-list[id=po10]",
				},
				gnmiext.DeletingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=po10]",
				},
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/intf-items/aggr-items/AggrIf-list[id=po99]",
				},
				gnmiext.DeletingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=po99]",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPortChannel(tt.pcName, tt.options...)
			if err != nil {
				t.Fatalf("failed to create port-channel: %v", err)
			}
			updates, err := p.Reset(t.Context(), nil)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
