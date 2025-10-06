// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

// AddressFamily represents the identifier of an address family.
type AddressFamily string

const (
	AddressFamilyL2EVPN      AddressFamily = "l2vpn-evpn"
	AddressFamilyIPv4Unicast AddressFamily = "ipv4-ucast"
	AddressFamilyIPv6Unicast AddressFamily = "ipv6-ucast"
)
