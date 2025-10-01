// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

func init() {
	name := "TwentyFiveGigE0/0/0/14"

	mtu := MTU{
		MTU:   9026,
		Owner: "TwentyFiveGigE",
	}

	Register("intf", &PhisIf{
		Name:        name,
		Description: "test",
		Active:      "act",
		MTUs: MTUs{
			[]MTU{mtu},
		},
		IPv4Network: IPv4Network{
			Addresses: AddressesIPv4{
				Primary: Primary{
					Address: "192.168.1.2",
					Netmask: "255.255.255.0",
				},
			},
		},
	})
}
