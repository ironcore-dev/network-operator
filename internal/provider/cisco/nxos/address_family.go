// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

// AddressFamily represents the identifier of an address family.
type AddressFamily string

const (
	AddressFamilyL2EVPN      AddressFamily = "l2vpn-evpn"
	AddressFamilyIPv4Unicast AddressFamily = "ipv4-ucast"
	AddressFamilyIPv6Unicast AddressFamily = "ipv6-ucast"
)

func (af AddressFamily) ToAddressFamilyType() v1alpha1.BGPAddressFamilyType {
	switch af {
	case AddressFamilyL2EVPN:
		return v1alpha1.BGPAddressFamilyL2vpnEvpn
	case AddressFamilyIPv4Unicast:
		return v1alpha1.BGPAddressFamilyIpv4Unicast
	case AddressFamilyIPv6Unicast:
		return v1alpha1.BGPAddressFamilyIpv6Unicast
	default:
		return v1alpha1.BGPAddressFamilyType("")
	}
}
