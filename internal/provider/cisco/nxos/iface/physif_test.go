// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
)

const physIfVRFName = "test-vrf"

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

func Test_PhysIf_ToYGOT_BaseConfig(t *testing.T) {
	tests := []struct {
		name            string
		ifName          string
		options         []PhysIfOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:    "No additional base options",
			ifName:  "eth1/1",
			options: []PhysIfOption{WithDescription("this is a test")},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/4]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 then L3 test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/4]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						Unnumbered: ygot.String("lo0"),
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/5]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L3 then L2 test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_access,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/5]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
						Mode:    nxos.Cisco_NX_OSDevice_Stp_IfMode_edge,
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 trunk test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_trunk,
						NativeVlan:    ygot.String("vlan-100"),
						TrunkVlans:    ygot.String("10,20,30"),
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth2/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("L2 access test"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer2,
						Mode:          nxos.Cisco_NX_OSDevice_L1_Mode_access,
						AccessVlan:    ygot.String("vlan-10"),
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth2/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Descr:         ygot.String("test interface"),
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						Medium:        nxos.Cisco_NX_OSDevice_L1_Medium_p2p,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth3/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Descr:         ygot.String("test interface"),
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth3/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
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
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
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
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth3/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{
						Descr:         ygot.String("test interface"),
						AdminSt:       nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						Layer:         nxos.Cisco_NX_OSDevice_L1_Layer_Layer3,
						UserCfgdFlags: ygot.String("admin_layer,admin_state"),
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id=eth3/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
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
				t.Errorf("unexpected error during ToYGOT: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}

func Test_PhysIf_Reset(t *testing.T) {
	tests := []struct {
		name            string
		ifName          string
		exists          func(ctx context.Context, xpath string) (bool, error)
		options         []PhysIfOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:   "basic reset",
			ifName: "eth1/1",
			exists: func(ctx context.Context, xpath string) (bool, error) {
				if xpath == "System/stp-items/inst-items/if-items/If-list[id=eth1/1]" {
					return true, nil
				}
				return false, nil
			},
			options: []PhysIfOption{
				WithDescription("test interface"),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/1]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
				},
			},
		},
		{
			name:   "reset with L2 configuration",
			ifName: "eth1/2",
			exists: func(ctx context.Context, xpath string) (bool, error) {
				if xpath == "System/stp-items/inst-items/if-items/If-list[id=eth1/2]" {
					return true, nil
				}
				return false, nil
			},
			options: []PhysIfOption{
				WithDescription("L2 test interface"),
				WithPhysIfL2(&L2Config{}),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/phys-items/PhysIf-list[id=eth1/2]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList{},
				},
			},
		},
		{
			name:   "reset with L3 configuration",
			ifName: "eth1/3",
			exists: func(ctx context.Context, xpath string) (bool, error) { return false, nil },
			options: []PhysIfOption{
				WithDescription("L3 test interface"),
				WithPhysIfL3(&L3Config{
					medium:             L3MediumTypeP2P,
					unnumberedLoopback: "lo0",
				}),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
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

			m := &gnmiext.ClientMock{ExistsFunc: tt.exists}

			updates, err := p.Reset(t.Context(), m)
			if err != nil {
				t.Errorf("unexpected error during reset: %v", err)
			}

			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}

func Test_PhysIf_FromYGOT(t *testing.T) {
	tests := []struct {
		name        string
		input       PhysIf
		expected    PhysIf
		clientMock  *gnmiext.ClientMock
		expectError bool
	}{
		{
			name:     "valid input without VRF",
			input:    PhysIf{name: "eth1/1"},
			expected: PhysIf{name: "eth1/1", description: "test interface", adminSt: false},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
					i.Id = ygot.String("eth1/1")
					i.Descr = ygot.String("test interface")
					i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_down
					return nil
				},
			},
			expectError: false,
		},
		{
			name:     "valid input with VRF",
			input:    PhysIf{name: "eth1/1"},
			expected: PhysIf{name: "eth1/1", description: "test interface", adminSt: true, vrf: "management"},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
					i.Id = ygot.String("eth1/1")
					i.Descr = ygot.String("test interface")
					i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_up
					i.RtvrfMbrItems = &nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList_RtvrfMbrItems{
						TDn: ygot.String("System/inst-items/Inst-list[name=management]"),
					}
					return nil
				},
			},
			expectError: false,
		},
		{
			name:  "valid input with L2 config",
			input: PhysIf{name: "eth1/1"},
			expected: PhysIf{
				name:        "eth1/1",
				description: "L2 interface",
				adminSt:     true,
				l2: &L2Config{
					switchPort:   SwitchPortModeAccess,
					spanningTree: SpanningTreeModeEdge,
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					switch xpath {
					case "System/intf-items/phys-items/PhysIf-list[id=eth1/1]":
						i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
						i.Id = ygot.String("eth1/1")
						i.Descr = ygot.String("L2 interface")
						i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_up
						i.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer2
						i.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_access
						return nil
					case "System/stp-items/inst-items/if-items/If-list[id=eth1/1]":
						s := dest.(*nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList)
						s.Id = ygot.String("eth1/1")
						s.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_edge
						return nil
					default:
						return errors.New("unexpected xpath: " + xpath)
					}
				},
			},
			expectError: false,
		},
		{
			name:  "valid: L2 config with VLAN list in non ascending format",
			input: PhysIf{name: "eth1/1"},
			expected: PhysIf{
				name:        "eth1/1",
				description: "L2 interface",
				adminSt:     true,
				l2: &L2Config{
					switchPort:   SwitchPortModeTrunk,
					spanningTree: SpanningTreeModeTrunk,
					accessVlan:   0,
					nativeVlan:   100,
					allowedVlans: []uint16{10, 11, 12, 13, 14, 15, 20, 30, 31},
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					switch xpath {
					case "System/intf-items/phys-items/PhysIf-list[id=eth1/1]":
						i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
						i.Id = ygot.String("eth1/1")
						i.Descr = ygot.String("L2 interface")
						i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_up
						i.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer2
						i.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
						i.AccessVlan = ygot.String("vlan-0")
						i.NativeVlan = ygot.String("vlan-100")
						i.TrunkVlans = ygot.String("30-31,20,10-15")
						return nil
					case "System/stp-items/inst-items/if-items/If-list[id=eth1/1]":
						s := dest.(*nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList)
						s.Id = ygot.String("eth1/1")
						s.Mode = nxos.Cisco_NX_OSDevice_Stp_IfMode_trunk
						return nil
					default:
						return errors.New("unexpected xpath: " + xpath)
					}
				},
			},
			expectError: false,
		},
		{
			name:  "valid input with L3 config (unnumbered)",
			input: PhysIf{name: "eth1/1"},
			expected: PhysIf{
				name:        "eth1/1",
				description: "L3 interface",
				adminSt:     false,
				l3: &L3Config{
					medium:             L3MediumTypeP2P,
					addressingMode:     AddressingModeUnnumbered,
					unnumberedLoopback: "lo0",
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					switch xpath {
					case "System/intf-items/phys-items/PhysIf-list[id=eth1/1]":
						i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
						i.Id = ygot.String("eth1/1")
						i.Descr = ygot.String("L3 interface")
						i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_down
						i.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer3
						i.Medium = nxos.Cisco_NX_OSDevice_L1_Medium_p2p
						return nil
					case "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/1]":
						s := dest.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList)
						s.Id = ygot.String("eth1/1")
						s.Unnumbered = ygot.String("lo0")
						return nil
					default:
						return errors.New("unexpected xpath: " + xpath)
					}
				},
			},
			expectError: false,
		},
		{
			name:  "valid input with L3 config (with addresses sorted differently)",
			input: PhysIf{name: "eth1/1"},
			expected: PhysIf{
				name:        "eth1/1",
				description: "L3 interface",
				adminSt:     true,
				l3: &L3Config{
					medium:         L3MediumTypeBroadcast,
					addressingMode: AddressingModeNumbered,
					prefixesIPv4: []netip.Prefix{
						netip.MustParsePrefix("10.0.0.1/24"),
						netip.MustParsePrefix("192.168.1.1/24"),
					},
					prefixesIPv6: []netip.Prefix{
						netip.MustParsePrefix("2001:db8::1/64"),
					},
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					switch xpath {
					case "System/intf-items/phys-items/PhysIf-list[id=eth1/1]":
						i := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList)
						i.Id = ygot.String("eth1/1")
						i.Descr = ygot.String("L3 interface")
						i.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_up
						i.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_Layer3
						i.Medium = nxos.Cisco_NX_OSDevice_L1_Medium_broadcast
					case "System/ipv4-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/1]":
						s := dest.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList)
						s.Id = ygot.String("eth1/1")
						s.AddrItems = &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
							AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
								"192.168.1.1/24": {
									Addr: ygot.String("192.168.1.1/24"),
								},
								"10.0.0.0/24": {
									Addr: ygot.String("10.0.0.1/24"),
								},
							},
						}
					case "System/ipv6-items/inst-items/dom-items/Dom-list[name=default]/if-items/If-list[id=eth1/1]":
						s := dest.(*nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList)
						s.Id = ygot.String("eth1/1")
						s.AddrItems = &nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
							AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
								"2001:db8::/64": {
									Addr: ygot.String("2001:db8::1/64"),
								},
							},
						}
					default:
						return errors.New("unexpected xpath: " + xpath)
					}
					return nil
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Example: Call FromYGOT and check error (replace with actual logic)
			err := tt.input.FromYGOT(t.Context(), tt.clientMock)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
			}
			equals, err := tt.input.Equals(&tt.expected)
			if err != nil {
				t.Errorf("error during equality check: %v", err)
			}
			if !equals {
				t.Errorf("expected output %+v, got %+v", tt.expected.String(), tt.input.String())
			}
		})
	}
}

func Test_PhysIf_Equal(t *testing.T) {
	tests := []struct {
		name          string
		a             *PhysIf
		b             gnmiext.DeviceConf
		expectedError bool
		expectedEqual bool
	}{
		{
			name:          "equal: identical interfaces",
			a:             &PhysIf{name: "eth1/1", description: "desc", adminSt: true, mtu: 1500, vrf: "vrf1"},
			b:             &PhysIf{name: "eth1/1", description: "desc", adminSt: true, mtu: 1500, vrf: "vrf1"},
			expectedError: false,
			expectedEqual: true,
		},
		{
			name:          "not-equal: different interface values (description)",
			a:             &PhysIf{name: "eth1/1", description: "A", adminSt: true, mtu: 1500, vrf: "vrf1"},
			b:             &PhysIf{name: "eth1/1", description: "B", adminSt: true, mtu: 1500, vrf: "vrf1"},
			expectedError: false,
			expectedEqual: false,
		},
		{
			name:          "not-equal: different interface values (missing description)",
			a:             &PhysIf{name: "eth1/1", description: "A"},
			b:             &PhysIf{name: "eth1/1"},
			expectedError: false,
			expectedEqual: false,
		},
		{
			name:          "error: different type",
			a:             &PhysIf{name: "eth1/1", description: "desc", adminSt: true},
			b:             &Loopback{name: "lo1", description: ygot.String("desc"), adminSt: true},
			expectedError: true,
			expectedEqual: false,
		},
		{
			name:          "equal: L2 with VLANs in different order",
			a:             &PhysIf{name: "eth1/1", l2: &L2Config{switchPort: SwitchPortModeAccess, spanningTree: SpanningTreeModeEdge, accessVlan: 10, nativeVlan: 40, allowedVlans: []uint16{20, 30, 40}}},
			b:             &PhysIf{name: "eth1/1", l2: &L2Config{switchPort: SwitchPortModeAccess, spanningTree: SpanningTreeModeEdge, accessVlan: 10, nativeVlan: 40, allowedVlans: []uint16{30, 40, 20}}},
			expectedError: false,
			expectedEqual: true,
		},
		{
			name:          "equal: L3 with IPs in different order",
			a:             &PhysIf{name: "eth1/1", l3: &L3Config{addressingMode: AddressingModeNumbered, prefixesIPv4: []netip.Prefix{netip.MustParsePrefix("192.168.1.1/24"), netip.MustParsePrefix("10.0.0.1/24")}}},
			b:             &PhysIf{name: "eth1/1", l3: &L3Config{addressingMode: AddressingModeNumbered, prefixesIPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24"), netip.MustParsePrefix("192.168.1.1/24")}}},
			expectedError: false,
			expectedEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equal, err := tt.a.Equals(tt.b)
			if err != nil && tt.expectedError == false {
				t.Errorf("unexpected error: %v", err)
			}
			if equal != tt.expectedEqual {
				t.Errorf("Equal() = %v, want %v", equal, tt.expectedEqual)
			}
		})
	}
}
