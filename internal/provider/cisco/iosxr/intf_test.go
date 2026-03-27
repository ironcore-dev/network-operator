// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import "testing"

func init() {
	name := "TwentyFiveGigE0/0/0/14"

	mtu := MTU{
		MTU:   9026,
		Owner: "TwentyFiveGigE",
	}

	Register("intf", &Iface{
		Name:        name,
		Description: "random interface test",
		Active:      "act",
		Vrf:         "default",
		Statistics: Statistics{
			LoadInterval: 30,
		},
		MTUs: MTUs{
			[]MTU{mtu},
		},
		Shutdown: true,
		IPv4Network: IPv4Network{
			Addresses: AddressesIPv4{
				Primary: Primary{
					Address: "192.168.1.2",
					Netmask: "255.255.255.0",
				},
			},
			Mtu: 1000,
		},
		IPv6Network: IPv6Network{
			Mtu: 2100,
			Addresses: AddressesIPv6{
				RegularAddresses: RegularAddresses{
					RegularAddress: []RegularAddress{
						{
							Address:      "2001:db8::1",
							PrefixLength: 64,
							Zone:         "",
						},
					},
				},
			},
		},
		IPv6Neighbor: IPv6Neighbor{
			RASuppress: true,
		},
	})
}

func TestValidateInterfaceName(t *testing.T) {
	tests := []struct {
		name      string
		ifaceName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid TenGigE interface",
			ifaceName: "TenGigE0001",
			wantErr:   false,
		},
		{
			name:      "valid TenGigE interface",
			ifaceName: "TenGigE0001.100",
			wantErr:   false,
		},
		{
			name:      "missing ",
			ifaceName: "eth-1-1",
			wantErr:   true,
		},
		{
			name:      "valid Bundle-Ether interface",
			ifaceName: "Bundle-Ether1",
			wantErr:   false,
		},
		{
			name:      "valid Bundle-Ether with VLAN",
			ifaceName: "Bundle-Ether1.100",
			wantErr:   false,
		},
		{
			name:      "valid BE interface",
			ifaceName: "BE1",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.ifaceName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Interface name %s accepted as valid, expected error", tt.ifaceName)
				}
			} else {
				if err != nil {
					t.Errorf("Interface name %s rejected as invalid, expected valid. Error: %v", tt.ifaceName, err)
				}
			}
		})
	}
}
