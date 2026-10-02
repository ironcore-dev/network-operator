// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	metric30 := int32(30)
	metric15 := int32(15)

	route := &StaticRoute{
		VRF:    "test-vrf",
		Prefix: "172.16.0.0/16",
	}
	route.NhItems.NexthopList.Set(NewStaticRouteNexthop("12.0.100.1", "test-vrf", "eth1/1", &metric30))
	route.NhItems.NexthopList.Set(NewStaticRouteNexthop("11.0.100.1", "test-vrf", "", &metric15))
	Register("static_route", route)
}
