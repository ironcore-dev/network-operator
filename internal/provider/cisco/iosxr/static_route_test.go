// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

func init() {
	metric12 := int32(12)
	metric10 := int32(10)
	metric9 := int32(9)

	route := &Prefix{
		PrefixAddress: "192.168.1.0",
		PrefixLength:  24,
		IsIpv4:        true,
		NextHopAddress: &NexthopAddresses{
			NexthopAddress: []NexthopAddress{
				NewNexthopAddress("10.10.0.1", &metric12),
			},
		},
		NextHopInterface: &NexthopInterfaces{
			NexthopInterface: []NexthopInterface{
				NewNexthopInterface("TwentyFiveGigE0/0/0/34", "10.9.2.1", &metric10),
				NewNexthopInterface("TwentyFiveGigE0/0/0/35", "10.8.1.1", &metric9),
			},
		},
	}

	Register("static_route", route)
}
