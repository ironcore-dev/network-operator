// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
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
		options         []PhysIfOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:   "basic reset",
			ifName: "eth1/1",
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
			options: []PhysIfOption{
				WithDescription("L3 test interface"),
				WithPhysIfL3(&L3Config{
					medium:             L3MediumTypeP2P,
					unnumberedLoopback: "lo0",
				}),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/stp-items/inst-items/if-items/If-list[id=eth1/3]",
					Value: &nxos.Cisco_NX_OSDevice_System_StpItems_InstItems_IfItems_IfList{},
				},
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

			updates, err := p.Reset(context.Background(), nil)
			if err != nil {
				t.Errorf("unexpected error during reset: %v", err)
			}

			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
