// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("loopback", &Loopback{
		ID:            "lo0",
		Descr:         "Test",
		AdminSt:       AdminStUp,
		RtvrfMbrItems: NewVrfMember("lo0", "management"),
	})

	Register("physif_rtd", &PhysIf{
		AdminSt:       AdminStUp,
		ID:            "eth1/1",
		Descr:         "Leaf1 to Spine1",
		Layer:         Layer3,
		MTU:           9216,
		Medium:        MediumPointToPoint,
		Mode:          SwitchportModeAccess,
		AccessVlan:    DefaultVLAN,
		NativeVlan:    DefaultVLAN,
		TrunkVlans:    DefaultVLANRange,
		UserCfgdFlags: "admin_layer,admin_mtu,admin_state",
	})

	Register("physif_switchport", &PhysIf{
		AdminSt:       AdminStUp,
		ID:            "eth1/10",
		Descr:         "Leaf1 to Host1",
		Layer:         Layer2,
		Medium:        MediumBroadcast,
		Mode:          SwitchportModeTrunk,
		AccessVlan:    DefaultVLAN,
		NativeVlan:    DefaultVLAN,
		TrunkVlans:    "10",
		UserCfgdFlags: "admin_state",
	})

	intfAddr4 := &AddrItem{ID: "lo0"}
	intfAddr4.AddrItems.AddrList.Set(&IntfAddr{
		Addr: "10.0.0.10/32",
		Pref: 0,
		Tag:  0,
		Type: "primary",
	})
	Register("intf_addr4", intfAddr4)

	pc := &PortChannel{
		AccessVlan:    DefaultVLAN,
		AdminSt:       AdminStUp,
		Descr:         "vPC Leaf1 to Host1",
		ID:            "po10",
		Layer:         Layer2,
		Mode:          SwitchportModeTrunk,
		PcMode:        PortChannelModeActive,
		NativeVlan:    DefaultVLAN,
		TrunkVlans:    "10",
		UserCfgdFlags: "admin_state",
	}
	pc.RsmbrIfsItems.RsMbrIfsList.Set(NewPortChannelMember("eth1/10"))
	Register("pc", pc)
}
