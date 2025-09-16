// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nve

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
)

func Test_NewNVE(t *testing.T) {
	tests := []struct {
		name        string
		options     []NVEOption
		shouldError bool
	}{
		{
			name:        "valid: default NVE with no options",
			options:     nil,
			shouldError: false,
		},
		{
			name:        "valid: set admin state to false",
			options:     []NVEOption{WithAdminState(false)},
			shouldError: false,
		},
		{
			name:        "valid: set advertise virtual rmac to true",
			options:     []NVEOption{WithAdvertiseVirtualRmac(true)},
			shouldError: false,
		},
		{
			name:        "valid: set advertise virtual rmac to false",
			options:     []NVEOption{WithAdvertiseVirtualRmac(false)},
			shouldError: false,
		},
		{
			name:        "valid: set host reachability protocol to BGP",
			options:     []NVEOption{WithHostReachabilityProtocol(HostReachBGP)},
			shouldError: false,
		},
		{
			name:        "valid: set host reachability protocol to Flood and Learn",
			options:     []NVEOption{WithHostReachabilityProtocol(HostReachFloodAndLearn)},
			shouldError: false,
		},
		{
			name:        "valid: set host reachability protocol to random",
			options:     []NVEOption{WithHostReachabilityProtocol(1000)},
			shouldError: true,
		},
		{
			name:        "valid: set source interface",
			options:     []NVEOption{WithSourceInterface("loopback0")},
			shouldError: false,
		},
		{
			name:        "invalid: set invalid source interface name",
			options:     []NVEOption{WithSourceInterface("eth1/1")},
			shouldError: true,
		},
		{
			name:        "invalid: set invalid source interface number",
			options:     []NVEOption{WithSourceInterface("loopback1024")},
			shouldError: true,
		},
		{
			name:        "invalid: set anycast interface without source interface",
			options:     []NVEOption{WithAnycastInterface("loopback1")},
			shouldError: true,
		},
		{
			name:        "valid: set source and anycast interfaces",
			options:     []NVEOption{WithSourceInterface("loopback0"), WithAnycastInterface("loopback1")},
			shouldError: false,
		},
		{
			name:        "invalid: set same source and anycast interfaces",
			options:     []NVEOption{WithSourceInterface("loopback0"), WithAnycastInterface("loopback0")},
			shouldError: true,
		},
		{
			name:        "invalid: source interface is correct but anycast is not",
			options:     []NVEOption{WithSourceInterface("loopback0"), WithAnycastInterface("loopback1900")},
			shouldError: true,
		},
		{
			name: "valid: set all options required by example config sample",
			options: []NVEOption{
				WithAdminState(false),
				WithSourceInterface("loopback0"),
				WithAnycastInterface("loopback1"),
				WithHostReachabilityProtocol(HostReachBGP),
				WithAdvertiseVirtualRmac(true),
				WithMulticastGroupL2("238.0.0.1"),
				WithHoldDownTime(300),
				WithSuppressARP(true),
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewNVE(tt.options...)
			if tt.shouldError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func Test_NVE_ToYGOT(t *testing.T) {
	tests := []struct {
		name            string
		options         []NVEOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:    "valid: default NVE",
			options: nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					Value: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
					},
				},
			},
		},
		{
			name:    "valid: set admin state to disabled",
			options: []NVEOption{WithAdminState(false)},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					Value: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled,
					},
				},
			},
		},
		{
			name: "valid: disabled but configured interface",
			options: []NVEOption{
				WithAdminState(false),
				WithHostReachabilityProtocol(HostReachFloodAndLearn),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					Value: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt:   nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled,
						HostReach: nxos.Cisco_NX_OSDevice_Nvo_HostReachT_Flood_and_learn,
					},
				},
			},
		},
		{
			name: "valid: don't advertise virtual rmac",
			options: []NVEOption{
				WithAdvertiseVirtualRmac(false),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					Value: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt:       nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
						AdvertiseVmac: ygot.Bool(false),
					},
				},
			},
		},
		{
			name: "valid: full sample configuration",
			options: []NVEOption{
				WithSourceInterface("loopback0"),
				WithAnycastInterface("loopback1"),
				WithSuppressARP(true),
				WithHostReachabilityProtocol(HostReachBGP),
				WithAdvertiseVirtualRmac(true),
				WithMulticastGroupL2("237.0.0.1"),
				WithMulticastGroupL3("238.0.0.1"),
				WithHoldDownTime(300),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/ngmvpn-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					Value: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt:         nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
						SourceInterface: ygot.String("lo0"),
						AnycastIntf:     ygot.String("lo1"),
						SuppressARP:     ygot.Bool(true),
						HostReach:       nxos.Cisco_NX_OSDevice_Nvo_HostReachT_bgp,
						AdvertiseVmac:   ygot.Bool(true),
						McastGroupL2:    ygot.String("237.0.0.1"),
						McastGroupL3:    ygot.String("238.0.0.1"),
						HoldDownTime:    ygot.Uint16(300),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nve, err := NewNVE(tt.options...)
			if err != nil {
				t.Fatalf("unexpected error during NewNVE: %v", err)
			}
			updates, err := nve.ToYGOT(context.TODO(), &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return true, nil },
			})
			if err != nil {
				t.Fatalf("unexpected error during ToYGOT: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}

func Test_NVE_Reset(t *testing.T) {
	tests := []struct {
		name            string
		options         []NVEOption
		expectedUpdates []gnmiext.Update
	}{
		{
			name:    "valid: reset default NVE",
			options: nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
				},
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/ngmvpn-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
			},
		},
		{
			name: "valid: reset NVE with some options",
			options: []NVEOption{
				WithSourceInterface("loopback0"),
				WithAnycastInterface("loopback1"),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
				},
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/ngmvpn-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				gnmiext.EditingUpdate{
					XPath: "/System/fm-items/nvo-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nve, err := NewNVE(tt.options...)
			if err != nil {
				t.Fatalf("unexpected error during NewNVE: %v", err)
			}
			updates, err := nve.Reset(context.TODO(), &gnmiext.ClientMock{
				ExistsFunc: func(_ context.Context, _ string) (bool, error) { return true, nil },
			})
			if err != nil {
				t.Fatalf("unexpected error during Reset: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
