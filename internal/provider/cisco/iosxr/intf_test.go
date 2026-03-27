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

func TestExtractBundleAndSubinterfaceID(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedBundleID   int32
		expectedSubIfaceID int32
		wantErr            bool
	}{
		{
			name:               "Bundle-Ether with subinterface",
			input:              "Bundle-Ether200.4095",
			expectedBundleID:   200,
			expectedSubIfaceID: 4095,
			wantErr:            false,
		},
		{
			name:               "Bundle-Ether with subinterface",
			input:              "Bundle-Ether200",
			expectedBundleID:   200,
			expectedSubIfaceID: 0,
			wantErr:            false,
		},
		{
			name:               "Bundle-Ether with subinterface",
			input:              "Bundle-Ether200.100.100",
			expectedBundleID:   0,
			expectedSubIfaceID: 0,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundleID, subIfaceID, err := ExtractBundleAndSubinterfaceID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractBundleAndSubinterfaceId(%s) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ExtractBundleAndSubinterfaceId(%s) unexpected error: %v", tt.input, err)
				}
				if bundleID != tt.expectedBundleID {
					t.Errorf("ExtractBundleAndSubinterfaceId(%s) bundleID = %v, want %v", tt.input, bundleID, tt.expectedBundleID)
				}
				if subIfaceID != tt.expectedSubIfaceID {
					t.Errorf("ExtractBundleAndSubinterfaceId(%s) subIfaceID = %v, want %v", tt.input, subIfaceID, tt.expectedSubIfaceID)
				}
			}
		})
	}
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
			ifaceName: "TenGigE0/0/0/1",
			wantErr:   false,
		},
		{
			name:      "valid TenGigE interface",
			ifaceName: "TenGigE0/0/0/1.100",
			wantErr:   false,
		},
		{
			name:      "invalid interface ios xr interface name",
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
			name:      "invalid BE interface",
			ifaceName: "BE1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.ifaceName)
			if tt.wantErr && err == nil {
				t.Errorf("Interface name %s accepted as valid, expected error", tt.ifaceName)
			} else if !tt.wantErr && err != nil {
				t.Errorf("Interface name %s rejected as invalid, expected valid. Error: %v", tt.ifaceName, err)
			}
		})
	}
}

func TestExtractInterfaceOwnerFromName(t *testing.T) {
	tests := []struct {
		name          string
		ifaceName     string
		expectedOwner IFaceOwner
		wantErr       bool
	}{
		{
			name:          "TF short form for TwentyFiveGigE",
			ifaceName:     "TwentyFiveGigE0/0/0/33",
			expectedOwner: Speed25G,
			wantErr:       false,
		},
		{
			name:          "TF short form for TwentyFiveGigE",
			ifaceName:     "TF0/0/0/33",
			expectedOwner: "",
			wantErr:       false,
		},
		{
			name:          "Loopback interface",
			ifaceName:     "Loopback0",
			expectedOwner: LoopBack,
			wantErr:       false,
		},
		{
			name:          "Management Interface",
			ifaceName:     "MgmtEth0/RP0/CPU0/0",
			expectedOwner: MgmtEth,
			wantErr:       false,
		},
		// Invalid interface name
		{
			name:          "Invalid interface name",
			ifaceName:     "InvalidInterface",
			expectedOwner: "",
			wantErr:       true,
		},
		{
			name:          "Invalid subinterface name",
			ifaceName:     "TF0/0/0/33.100.100",
			expectedOwner: "",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, err := ExtractOwnerFromInterfaceName(tt.ifaceName)
			if tt.wantErr && err == nil {
				t.Errorf("ExtractOwnerFromInterfaceName(%s) expected error, got nil", tt.ifaceName)
			}
			if owner != tt.expectedOwner {
				t.Errorf("ExtractOwnerFromInterfaceName(%s) = %v, want %v", tt.ifaceName, owner, tt.expectedOwner)
			}
		})
	}
}
