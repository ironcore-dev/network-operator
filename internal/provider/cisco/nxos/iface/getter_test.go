// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"testing"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestExists(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		fn            func(ctx context.Context, xpath string) (bool, error)
		wantExists    bool
		wantErr       bool
	}{
		// Valid Ethernet interface cases
		{
			name:          "ethernet interface exists - full name",
			interfaceName: "Ethernet1/1",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/phys-items/PhysIf-list[id=eth1/1]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "ethernet interface exists - short name",
			interfaceName: "eth10/24",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/phys-items/PhysIf-list[id=eth10/24]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "ethernet interface exists but has no value - ErrNil",
			interfaceName: "Ethernet2/2",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/phys-items/PhysIf-list[id=eth2/2]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:          "ethernet interface does not exist - ErrNotFound",
			interfaceName: "eth3/3",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/phys-items/PhysIf-list[id=eth3/3]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},

		// Valid Loopback interface cases
		{
			name:          "loopback interface exists - full name",
			interfaceName: "Loopback1",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/lb-items/LbRtdIf-list[id=lo1]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "loopback interface exists - short name",
			interfaceName: "lo100",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/lb-items/LbRtdIf-list[id=lo100]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "loopback interface does not exist - ErrNil",
			interfaceName: "Loopback2",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/lb-items/LbRtdIf-list[id=lo2]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:          "loopback interface does not exist - ErrNotFound",
			interfaceName: "lo3",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/lb-items/LbRtdIf-list[id=lo3]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:          "port-channel exists - full name",
			interfaceName: "Port-Channel10",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/aggr-items/AggrIf-list[id=po10]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "port-channel exists - short name",
			interfaceName: "po20",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/aggr-items/AggrIf-list[id=po20]" {
					return true, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:          "port-channel does not exist - ErrNil",
			interfaceName: "Port-Channel30",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/aggr-items/AggrIf-list[id=po30]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:          "port-channel does not exist - ErrNotFound",
			interfaceName: "po40",
			fn: func(_ context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/aggr-items/AggrIf-list[id=po40]" {
					return false, nil
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    false,
		},

		// Error cases
		{
			name:          "empty interface name",
			interfaceName: "",
			fn: func(ctx context.Context, xpath string) (bool, error) {
				return false, errors.New("should not be called")
			},
			wantExists: false,
			wantErr:    true,
		},
		{
			name:          "unsupported interface format",
			interfaceName: "Foobar1/1",
			fn: func(ctx context.Context, xpath string) (bool, error) {
				return false, errors.New("should not be called")
			},
			wantExists: false,
			wantErr:    true,
		},
		{
			name:          "client error",
			interfaceName: "Ethernet1/1",
			fn: func(ctx context.Context, xpath string) (bool, error) {
				if xpath == "System/intf-items/phys-items/PhysIf-list[id=eth1/1]" {
					return false, errors.New("connection failed")
				}
				return false, errors.New("unexpected xpath")
			},
			wantExists: false,
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := &gnmiext.ClientMock{ExistsFunc: test.fn}
			exists, err := Exists(context.Background(), mock, test.interfaceName)
			if exists != test.wantExists {
				t.Errorf("Exists(%q) = %v, want %v", test.interfaceName, exists, test.wantExists)
			}
			if test.wantErr {
				if err == nil {
					t.Errorf("Exists(%q) expected error, but got none", test.interfaceName)
				}
			} else {
				if err != nil {
					t.Errorf("Exists(%q) unexpected error: %v", test.interfaceName, err)
				}
			}
		})
	}
}
