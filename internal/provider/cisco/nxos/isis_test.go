// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "strings"

func init() {
	dom := &ISISDom{
		Name:        DefaultVRFName,
		Net:         "49.0001.0000.0000.0010.00",
		IsType:      ISISLevel1,
		PassiveDflt: ISISLevel1,
	}
	dom.AfItems.DomAfList = []*ISISDomAf{
		{Type: ISISAfIPv6Unicast},
		{Type: ISISAfIPv4Unicast},
	}
	dom.OverloadItems.AdminSt = "bootup"
	dom.OverloadItems.BgpAsNumStr = "none"
	dom.OverloadItems.StartupTime = 61
	for _, name := range []string{"lo1", "lo0", "eth1/2", "eth1/1"} {
		intf := &ISISInterface{
			ID:             name,
			NetworkTypeP2P: AdminStOff,
			V4Enable:       true,
			V4Bfd:          "enabled",
			V6Enable:       true,
			V6Bfd:          "enabled",
		}
		if strings.HasPrefix(name, "eth") {
			intf.NetworkTypeP2P = AdminStOn
		}
		dom.IfItems.IfList = append(dom.IfItems.IfList, intf)
	}
	isis := &ISIS{Name: "UNDERLAY", AdminSt: AdminStEnabled}
	isis.DomItems.DomList = []*ISISDom{dom}
	Register("isis", isis)
}
