// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("vlan", &VLAN{AdminSt: BdStateActive, BdState: BdStateActive, FabEncap: "vlan-10", Name: NewOption("Test")})
	Register("vlan_reservation", &VLANReservation{SysVlan: 3850})
	Register("vlan_system", &VLANSystem{LongName: true})
}
