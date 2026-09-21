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

	// IPSLA-enabled static route: EnsureStaticRoute emits a track, a matching
	// IPSLA operation, and its schedule per unique nexthop when Spec.IPSLA is true.
	track := &Track{
		TrackID: "svenvrf-192_168_1_0-10_10_0_1",
		Type:    TrackType{RTR: 100},
	}
	Register("track", track)

	ipslaOp := &IPSLAOperation{
		OperationNumber: 100,
		Type: &IPSLAOperationType{
			ICMP: &IPSLAICMP{
				Echo: &IPSLAEcho{
					Destination: &IPSLADestination{
						Address: IPSLAAddress{IPv4Address: "10.10.0.1"},
					},
					VRF:       "svenvrf",
					Frequency: IPSLAFrequency,
				},
			},
		},
	}
	Register("ipsla", ipslaOp)

	ipslaSchedule := &IPSLASchedule{
		OperationNumber: 100,
		Life:            &IPSLALife{Forever: &struct{}{}},
		StartTime:       &IPSLAStartTime{Now: &struct{}{}},
	}
	Register("ipsla_schedule", ipslaSchedule)
}
