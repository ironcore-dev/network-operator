// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	dom := &ISISDom{
		Name:        DefaultVRFName,
		Net:         "49.0001.0000.0000.0010.00",
		IsType:      ISISLevel1,
		PassiveDflt: ISISLevel1,
	}
	dom.AfItems.DomAfList.Set(&ISISDomAf{Type: ISISAfIPv4Unicast})
	dom.OverloadItems.AdminSt = "bootup"
	dom.OverloadItems.BgpAsNumStr = "none"
	dom.OverloadItems.StartupTime = 61
	dom.IfItems.IfList.Set(&ISISInterface{
		ID:             "eth1/1",
		NetworkTypeP2P: AdminStOn,
		V4Enable:       true,
		V4Bfd:          "enabled",
		V6Enable:       true,
		V6Bfd:          "enabled",
	})
	isis := &ISIS{Name: "UNDERLAY", AdminSt: AdminStEnabled}
	isis.DomItems.DomList.Set(dom)
	Register("isis", isis)
}
