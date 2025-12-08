// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("bgw", &MultisiteItems{SiteID: "1", AdminSt: AdminStEnabled, DelayRestoreSeconds: 180})

	sc := new(StormControlItems)
	sc.EvpnStormControlList.Set(&StormControlItem{Floatlevel: "0.100000", Name: StormControlTypeBroadcast})
	Register("storm_ctrl", sc)
}
