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
				NewNexthopAddress("10.10.0.1", &metric12, nil),
			},
		},
		NextHopInterface: &NexthopInterfaces{
			NexthopInterface: []NexthopInterface{
				NewNexthopInterface("TwentyFiveGigE0/0/0/34", "10.9.2.1", &metric10, nil),
				NewNexthopInterface("TwentyFiveGigE0/0/0/35", "10.8.1.1", &metric9, nil),
			},
		},
	}

	Register("static_route", route)

	fastDetect := &BFD{FastDetect: &FastDetect{MinimumInterval: 300, Multiplier: 3}}
	bfdRoute := &Prefix{
		PrefixAddress: "192.168.2.0",
		PrefixLength:  24,
		IsIpv4:        true,
		NextHopAddress: &NexthopAddresses{
			NexthopAddress: []NexthopAddress{
				NewNexthopAddress("10.10.0.1", &metric12, fastDetect),
			},
		},
		NextHopInterface: &NexthopInterfaces{
			NexthopInterface: []NexthopInterface{
				NewNexthopInterface("TwentyFiveGigE0/0/0/34", "10.9.2.1", &metric10, fastDetect),
			},
		},
	}

	Register("static_route_bfd", bfdRoute)
}
