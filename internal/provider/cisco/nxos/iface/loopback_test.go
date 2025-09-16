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

func Test_NewLoopback(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{"valid: Loopback0", "Loopback0", false},
		{"valid: loopback123", "loopback123", false},
		{"valid: lo1", "lo1", false},
		{"valid: lo99", "lo99", false},
		{"invalid: test", "test", true},
		{"invalid: Loopback", "Loopback", true},
		{"invalid: lo", "lo", true},
		{"invalid: Loopback1/2", "Loopback1/2", true},
		{"invalid: lo1.1", "lo1.1", true},
		{"invalid: eth100", "eth100", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLoopbackInterface(tt.input, nil)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
		})
	}
}

func Test_Loopback_ToYGOT_BaseConfig(t *testing.T) {
	tests := []struct {
		name            string
		inputName       string
		description     string
		options         []LoopbackOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:        "No additional base options",
			inputName:   "Loopback0",
			description: "Test Loopback Interface",
			options:     nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/lb-items/LbRtdIf-list[id=lo0]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
						Descr:   ygot.String("Test Loopback Interface"),
						AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
					},
				},
			},
		},
		{
			name:        "With VRF",
			inputName:   "Loopback0",
			description: "Test Loopback Interface",
			options:     []LoopbackOption{WithLoopbackVRF("test-vrf")},
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/lb-items/LbRtdIf-list[id=lo0]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
						Descr:   ygot.String("Test Loopback Interface"),
						AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewLoopbackInterface(tt.inputName, &tt.description, tt.options...)
			if err != nil {
				t.Fatalf("failed to create loopback interface: %v", err)
			}
			got, err := p.ToYGOT(context.Background(), &gnmiext.ClientMock{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			testutils.AssertEqual(t, got, tt.expectedUpdates)
		})
	}
}

func Test_Loopback_ToYGOT_WithL3Config(t *testing.T) {

	testAddressingL3Cfg, err := NewL3Config(
		WithNumberedAddressingIPv4([]string{"10.0.0.1/24"}),
	)
	if err != nil {
		panic(err)
	}
	testAddressingOptions := []LoopbackOption{
		WithLoopbackL3(testAddressingL3Cfg),
		WithLoopbackVRF("test-vrf"),
	}

	tests := []struct {
		name            string
		l3cfg           *L3Config
		options         []LoopbackOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:    "Addressing",
			l3cfg:   testAddressingL3Cfg,
			options: testAddressingOptions,
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/intf-items/lb-items/LbRtdIf-list[id=lo0]",
					Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
						Descr:   ygot.String("Test Loopback Interface"),
						AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
						RtvrfMbrItems: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList_RtvrfMbrItems{
							TDn: ygot.String("System/inst-items/Inst-list[name=test-vrf]"),
						},
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=test-vrf]/if-items/If-list[id=lo0]",
					Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{
						AddrItems: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems{
							AddrList: map[string]*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
								"10.0.0.1/24": {
									Addr: ygot.String("10.0.0.1/24"),
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
			p, err := NewLoopbackInterface("Loopback0", ygot.String("Test Loopback Interface"), tt.options...)
			if err != nil {
				t.Fatalf("failed to create loopback interface: %v", err)
			}
			got, err := p.ToYGOT(context.Background(), &gnmiext.ClientMock{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			testutils.AssertEqual(t, got, tt.expectedUpdates)
		})
	}
}

func Test_Loopback_ToYGOT_InvalidL3Config(t *testing.T) {
	tests := []struct {
		name    string
		options []LoopbackOption
	}{
		{
			name: "With unnumbered addressing",
			options: []LoopbackOption{
				WithLoopbackL3(func() *L3Config {
					l3cfg, err := NewL3Config(
						WithMedium(L3MediumTypeP2P),
						WithUnnumberedAddressing("loopback1"),
					)
					if err != nil {
						panic(err)
					}
					return l3cfg
				}()),
			},
		},
		{
			name: "With medium only",
			options: []LoopbackOption{
				WithLoopbackL3(func() *L3Config {
					l3cfg, err := NewL3Config(
						WithMedium(L3MediumTypeP2P),
					)
					if err != nil {
						panic(err)
					}
					return l3cfg
				}()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLoopbackInterface("Loopback0", ygot.String("Test Loopback Interface"), tt.options...)
			if err == nil {
				t.Fatalf("expected error for invalid L3 config, got nil")
			}
		})
	}
}

func Test_Loopback_Reset(t *testing.T) {
	tests := []struct {
		name            string
		inputName       string
		options         []LoopbackOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:      "basic reset",
			inputName: "Loopback0",
			options:   nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/intf-items/lb-items/LbRtdIf-list[id=lo0]",
				},
			},
		},
		{
			name:      "reset with VRF",
			inputName: "Loopback1",
			options:   []LoopbackOption{WithLoopbackVRF("test-vrf")},
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/intf-items/lb-items/LbRtdIf-list[id=lo1]",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewLoopbackInterface(tt.inputName, ygot.String("Test Loopback Interface"), tt.options...)
			if err != nil {
				t.Fatalf("failed to create loopback interface: %v", err)
			}
			updates, err := p.Reset(context.Background(), nil)
			if err != nil {
				t.Errorf("unexpected error during reset: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
