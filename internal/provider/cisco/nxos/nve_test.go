// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	nve := &NVE{
		ID:               1,
		AdminSt:          AdminStEnabled,
		HostReach:        HostReachBGP,
		AdvertiseVmac:    true,
		SourceInterface:  "lo0",
		AnycastInterface: "lo1",
		SuppressARP:      true,
		McastGroupL2:     "237.0.0.1",
		HoldDownTime:     300,
	}
	Register("nve", nve)
}
