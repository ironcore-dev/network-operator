// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import "strconv"

func (s *Prefix) XPath() string {
	basePath := "Cisco-IOS-XR-um-router-static-cfg:router/static/address-family/"
	if s.VRFName != "" {
		basePath = "Cisco-IOS-XR-um-router-static-cfg:router/static/vrfs/vrf[vrf-name=" + s.VRFName + "]/address-family/"
	}

	if s.IsIpv4 {
		return basePath + "ipv4/unicast/prefixes/prefix[prefix-address=" + s.PrefixAddress + "][prefix-length=" + strconv.Itoa(s.PrefixLength) + "]"
	}
	return basePath + "ipv6/unicast/prefixes/prefix[prefix-address=" + s.PrefixAddress + "][prefix-length=" + strconv.Itoa(s.PrefixLength) + "]"
}

type Prefix struct {
	PrefixAddress    string             `json:"prefix-address"`
	PrefixLength     int                `json:"prefix-length"`
	NextHopAddress   *NexthopAddresses  `json:"nexthop-addresses,omitempty"`
	NextHopInterface *NexthopInterfaces `json:"nexthop-interfaces,omitzero"`
	VRFName          string             `json:"-"`
	IsIpv4           bool               `json:"-"`
}

type NexthopAddresses struct {
	NexthopAddress []NexthopAddress `json:"nexthop-address"`
}

type NexthopAddress struct {
	Address  string `json:"address"`
	Distance uint32 `json:"distance-metric,omitempty"`
}

type NexthopInterfaces struct {
	NexthopInterface []NexthopInterface `json:"nexthop-interface,omitempty"`
}

type NexthopInterface struct {
	InterfaceName string `json:"interface-name,omitempty"`
	Distance      uint32 `json:"distance-metric,omitempty"`
}

func NewNexthopAddress(address string, distance *int32) NexthopAddress {
	nexthop := NexthopAddress{
		Address: address,
	}
	if distance != nil && *distance >= 0 {
		nexthop.Distance = uint32(*distance)
	}
	return nexthop
}

func NewNexthopInterface(name string, distance *int32) NexthopInterface {
	nexthop := NexthopInterface{
		InterfaceName: name,
	}
	if distance != nil && *distance >= 0 {
		nexthop.Distance = uint32(*distance)
	}
	return nexthop
}
