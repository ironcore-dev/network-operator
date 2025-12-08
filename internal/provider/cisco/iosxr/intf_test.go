// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

func init() {
	name := "TwentyFiveGigE0/0/0/14"

	mtu := MTU{
		MTU:   9026,
		Owner: "TwentyFiveGigE",
	}

	Register("intf", &PhysIf{
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
	})
}
