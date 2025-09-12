// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package isis

import (
	"context"
	"errors"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/openconfig/ygot/ygot"
)

func Test_Interface_NewInterface(t *testing.T) {
	tests := []struct {
		name      string
		ifName    string
		expectErr bool
	}{
		{
			name:      "Valid interface name Ethernet1/1",
			ifName:    "Ethernet1/1",
			expectErr: false,
		},
		{
			name:      "Valid interface name lo0",
			ifName:    "lo0",
			expectErr: false,
		},
		{
			name:      "Invalid interface name",
			ifName:    "invalid1/1",
			expectErr: true,
		},
		{
			name:      "Unallowed interface: port-channel",
			ifName:    "port-channel10",
			expectErr: true,
		},
		{
			name:      "Unallowed interface: management",
			ifName:    "mgmt0",
			expectErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewInterface(tc.ifName)
			if (err != nil) != tc.expectErr {
				t.Errorf("NewInterface(%q) expectErr=%v, got err=%v", tc.ifName, tc.expectErr, err)
			}
		})
	}
}
func Test_Interface_toYGOT(t *testing.T) {
	tests := []struct {
		name      string
		ifName    string
		opts      []IfOption
		exists    bool
		want      *nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList_DomItems_DomList_IfItems_IfList
		expectErr bool
	}{
		{
			name:   "Default IPv4/IPv6, interface exists",
			ifName: "Ethernet1/1",
			exists: true,
			want: &nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList_DomItems_DomList_IfItems_IfList{
				Id:             ygot.String("eth1/1"),
				V4Enable:       ygot.Bool(true),
				V6Enable:       ygot.Bool(true),
				NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_UNSET,
				V4Bfd:          0,
			},
		},
		{
			name:   "IPv6 only, P2P, BFD",
			ifName: "Ethernet1/2",
			opts:   []IfOption{WithIPv4(false), WithIPv6(true), WithPointToPoint(), WithBFD()},
			exists: true,
			want: &nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList_DomItems_DomList_IfItems_IfList{
				Id:             ygot.String("eth1/2"),
				V4Enable:       ygot.Bool(false),
				V6Enable:       ygot.Bool(true),
				NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_on,
				V4Bfd:          nxos.Cisco_NX_OSDevice_Isis_BfdT_enabled,
			},
		},
		{
			name:      "Interface does not exist",
			ifName:    "Ethernet1/4",
			exists:    false,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			intf, err := NewInterface(tc.ifName, tc.opts...)
			if err != nil {
				t.Fatalf("unexpected error from NewInterface: %v", err)
			}
			got, err := intf.toYGOT(context.Background(), &gnmiext.ClientMock{
				ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
					if tc.expectErr {
						return false, errors.New("error")
					}
					return tc.exists, nil
				},
			})

			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != nil {
				notification, err := ygot.Diff(got, tc.want)
				if err != nil {
					t.Errorf("failed to compute diff: %v", err)
				}
				if len(notification.Update) > 0 || len(notification.Delete) > 0 {
					t.Errorf("unexpected diff: %s", notification)
				}
			}
		})
	}
}
