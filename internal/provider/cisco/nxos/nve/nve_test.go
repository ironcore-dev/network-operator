// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nve

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
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

// updateCheck is used to validate updates in tests and helper functions.
type updateCheck struct {
	updateIdx   int    // the position we want to check in the returned slice of updates
	expectType  string // "EditingUpdate", "ReplacingUpdate", or "DeletingUpdate"
	expectXPath string // the expected XPath of the update
	expectValue any    // the expected ygot object that should be in the update
}

func Test_NVE_ToYGOT(t *testing.T) {
	tests := []struct {
		name                    string
		options                 []NVEOption
		expectedNumberOfUpdates int
		updateChecks            []updateCheck
	}{
		{
			name:                    "valid: default NVE",
			options:                 nil,
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/nvo-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
					},
				},
			},
		},
		{
			name:                    "valid: set admin state to disabled",
			options:                 []NVEOption{WithAdminState(false)},
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
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
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
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
			expectedNumberOfUpdates: 2,
			updateChecks: []updateCheck{
				{
					updateIdx:   1,
					expectType:  "ReplacingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
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
			expectedNumberOfUpdates: 3,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/ngmvpn-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				{
					updateIdx:   1,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/nvo-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				{
					updateIdx:   2,
					expectType:  "ReplacingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
					expectValue: &nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{
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

			if len(updates) != tt.expectedNumberOfUpdates {
				t.Fatalf("expected %d updates, got %d", tt.expectedNumberOfUpdates, len(updates))
			}

			validateUpdates(t, updates, tt.updateChecks)
		})
	}
}

func Test_NVE_Reset(t *testing.T) {
	tests := []struct {
		name                    string
		options                 []NVEOption
		expectedNumberOfUpdates int
		updateChecks            []updateCheck
	}{
		{
			name:                    "valid: reset default NVE",
			options:                 nil,
			expectedNumberOfUpdates: 3,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "DeletingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
				},
				{
					updateIdx:   1,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/ngmvpn-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				{
					updateIdx:   2,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/nvo-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
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
			expectedNumberOfUpdates: 3,
			updateChecks: []updateCheck{
				{
					updateIdx:   0,
					expectType:  "DeletingUpdate",
					expectXPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
				},
				{
					updateIdx:   1,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/ngmvpn-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
				{
					updateIdx:   2,
					expectType:  "EditingUpdate",
					expectXPath: "/System/fm-items/nvo-items",
					expectValue: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
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

			if len(updates) != tt.expectedNumberOfUpdates {
				t.Fatalf("expected %d updates, got %d", tt.expectedNumberOfUpdates, len(updates))
			}

			validateUpdates(t, updates, tt.updateChecks)
		})
	}
}

// validateUpdates is a helper function to validate updates against expected checks.
func validateUpdates(t *testing.T, updates []gnmiext.Update, checks []updateCheck) {
	for _, check := range checks {
		if check.updateIdx >= len(updates) {
			t.Errorf("missing update at index %d", check.updateIdx)
			continue
		}

		update := updates[check.updateIdx]
		var xpath string
		var value any

		switch u := update.(type) {
		case gnmiext.DeletingUpdate:
			if check.expectType != "DeletingUpdate" {
				t.Errorf("expected DeletingUpdate at index %d, got %T", check.updateIdx, update)
				continue
			}
			xpath = u.XPath
		case gnmiext.EditingUpdate:
			if check.expectType != "EditingUpdate" {
				t.Errorf("expected EditingUpdate at index %d, got %T", check.updateIdx, update)
				continue
			}
			xpath = u.XPath
			value = u.Value
		case gnmiext.ReplacingUpdate: // Handle ReplacingUpdate
			if check.expectType != "ReplacingUpdate" {
				t.Errorf("expected ReplacingUpdate at index %d, got %T", check.updateIdx, update)
				continue
			}
			xpath = u.XPath
			value = u.Value
		default:
			t.Errorf("unexpected update type at index %d: %T", check.updateIdx, update)
			continue
		}

		if xpath != check.expectXPath {
			t.Errorf("wrong xpath at index %d, expected '%s', got '%s'", check.updateIdx, check.expectXPath, xpath)
		}

		if check.expectValue != nil {
			compareYGOTValues(t, value, check.expectValue, check.updateIdx)
		}
	}
}

// compareYGOTValues is a helper function to compare two ygot.GoStruct values.
func compareYGOTValues(t *testing.T, actual, expected any, index int) {
	actualGoStruct, ok1 := actual.(ygot.GoStruct)
	expectedGoStruct, ok2 := expected.(ygot.GoStruct)
	if !ok1 || !ok2 {
		t.Errorf("failed to type assert value or expectValue to ygot.GoStruct at index %d", index)
		return
	}

	notification, err := ygot.Diff(actualGoStruct, expectedGoStruct)
	if err != nil {
		t.Errorf("failed to compute diff at index %d: %v", index, err)
		return
	}

	if len(notification.Update) > 0 || len(notification.Delete) > 0 {
		t.Errorf("unexpected diff at index %d: %s", index, notification)
	}
}
