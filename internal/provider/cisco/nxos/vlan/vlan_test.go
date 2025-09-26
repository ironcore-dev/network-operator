// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package vlan

import (
	"context"
	"errors"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
	"github.com/openconfig/ygot/ygot"
)

var clientMock = &gnmiext.ClientMock{
	GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
		destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems)
		if !ok {
			return errors.New("unexpected type in GetFunc mock")
		}
		destVal.SysVlan = ygot.Uint16(3968)
		return nil
	},
}

func TestVLAN_ToYGOT(t *testing.T) {
	tests := []struct {
		name            string
		v               VLAN
		clientMock      gnmiext.Client
		expectErr       bool
		expectedUpdates []gnmiext.Update
	}{
		{
			name:       "valid VLAN",
			v:          VLAN{ID: 10, Name: "test", AdminState: true},
			clientMock: clientMock,
			expectErr:  false,
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-10]",
					Value: &nxos.Cisco_NX_OSDevice_System_BdItems_BdItems_BDList{
						Id:      ygot.Uint32(10),
						Name:    ygot.String("test"),
						AdminSt: nxos.Cisco_NX_OSDevice_L2_DomAdminSt_active,
					},
				},
			},
		},
		{
			name:       "valid VLAN, no name, default state is admin down",
			v:          VLAN{ID: 20},
			clientMock: clientMock,
			expectErr:  false,
			expectedUpdates: []gnmiext.Update{
				gnmiext.ReplacingUpdate{
					XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-20]",
					Value: &nxos.Cisco_NX_OSDevice_System_BdItems_BdItems_BDList{
						Id:      ygot.Uint32(20),
						AdminSt: nxos.Cisco_NX_OSDevice_L2_DomAdminSt_suspend,
					},
				},
			},
		},
		{
			name:       "invalid: VLAN 0",
			v:          VLAN{ID: 0},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: VLAN 4096",
			v:          VLAN{ID: 4096},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to modify system VLAN 1",
			v:          VLAN{ID: 1},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to modify system VLAN 4093",
			v:          VLAN{ID: 4093},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to modify system VLAN 4094",
			v:          VLAN{ID: 4094},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to modify system VLAN 4095",
			v:          VLAN{ID: 4096},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: in system reserved range 3968-4095",
			v:          VLAN{ID: 4000},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to shutdown VLAN 1006 in extended range 1006-3967",
			v:          VLAN{ID: 1006},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to shutdown VLAN 3967 in extended range 1006-3967",
			v:          VLAN{ID: 3967},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name: "invalid: in system reserved range 100-268",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.SysVlan = ygot.Uint16(100)
					return nil
				},
			},
			expectErr: true,
		},
		{
			name: "invalid: failed to get reserved VLANs from device",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					return errors.New("gNMI error")
				},
			},
			expectErr: true,
		},
		{
			name: "invalid: no values returned for reserved VLANs from device",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.SysVlan = ygot.Uint16(0)
					return nil
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, err := tt.v.ToYGOT(t.Context(), tt.clientMock)
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

func TestVLAN_Reset(t *testing.T) {
	tests := []struct {
		name            string
		v               VLAN
		clientMock      gnmiext.Client
		expectErr       bool
		expectedUpdates []gnmiext.Update
	}{
		{
			name:       "valid VLAN reset in normal range",
			v:          VLAN{ID: 10, Name: "test", AdminState: true},
			clientMock: clientMock,
			expectErr:  false,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-10]",
				},
			},
		},
		{
			name:       "valid VLAN reset in extended range",
			v:          VLAN{ID: 2000, Name: "test", AdminState: true},
			clientMock: clientMock,
			expectErr:  false,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-2000]",
				},
			},
		},
		{
			name:       "invalid: VLAN 0",
			v:          VLAN{ID: 0},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: VLAN 4096",
			v:          VLAN{ID: 4096},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to reset system VLAN 1",
			v:          VLAN{ID: 1},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to reset system VLAN 4093",
			v:          VLAN{ID: 4093},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to reset system VLAN 4094",
			v:          VLAN{ID: 4094},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: attempt to reset system VLAN 4095",
			v:          VLAN{ID: 4095},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name:       "invalid: in system reserved range 3968-4095",
			v:          VLAN{ID: 4000},
			clientMock: clientMock,
			expectErr:  true,
		},
		{
			name: "invalid: in custom reserved range 100-268",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.SysVlan = ygot.Uint16(100)
					return nil
				},
			},
			expectErr: true,
		},
		{
			name: "invalid: failed to get reserved VLANs from device",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					return errors.New("gNMI error")
				},
			},
			expectErr: true,
		},
		{
			name: "invalid: no values returned for reserved VLANs from device",
			v:    VLAN{ID: 200},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.SysVlan = ygot.Uint16(0)
					return nil
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, err := tt.v.Reset(t.Context(), tt.clientMock)
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
