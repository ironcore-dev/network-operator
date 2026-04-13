// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("loopback", &Loopback{
		ID:            "lo0",
		Descr:         NewOption("Test"),
		AdminSt:       AdminStUp,
		RtvrfMbrItems: NewVrfMember("lo0", ManagementVRFName),
	})

	Register("physif_rtd", &PhysIf{
		AdminSt:       AdminStUp,
		ID:            "eth1/1",
		Descr:         NewOption("Leaf1 to Spine1"),
		FecMode:       FecModeAuto,
		Layer:         Layer3,
		MTU:           9216,
		Medium:        MediumPointToPoint,
		Mode:          SwitchportModeAccess,
		AccessVlan:    DefaultVLAN,
		NativeVlan:    DefaultVLAN,
		TrunkVlans:    DefaultVLANRange,
		UserCfgdFlags: UserFlagAdminState | UserFlagAdminLayer | UserFlagAdminMTU,
	})

	Register("physif_switchport", &PhysIf{
		AdminSt:       AdminStUp,
		ID:            "eth1/10",
		Descr:         NewOption("Leaf1 to Host1"),
		FecMode:       FecModeAuto,
		Layer:         Layer2,
		MTU:           DefaultMTU,
		Medium:        MediumBroadcast,
		Mode:          SwitchportModeTrunk,
		AccessVlan:    DefaultVLAN,
		NativeVlan:    DefaultVLAN,
		TrunkVlans:    "10",
		UserCfgdFlags: UserFlagAdminState,
	})

	Register("subinterface", &EncapRoutedInterface{
		ID:         "eth1/1.100",
		MTU:        1500,
		Medium:     MediumBroadcast,
		MTUInherit: false,
		Encap:      "100",
		AdminSt:    AdminStUp,
		Descr:      NewOption("L3 Subinterface on eth1/1"),
	})

	intfAddr4 := &AddrItem{ID: "lo0", Vrf: DefaultVRFName}
	intfAddr4.AddrItems.AddrList.Set(&IntfAddr{
		Addr: "10.0.0.10/32",
		Pref: 0,
		Tag:  0,
		Type: "primary",
	})
	Register("intf_addr4", intfAddr4)

	pc := &PortChannel{
		AccessVlan:     DefaultVLAN,
		AdminSt:        AdminStUp,
		Descr:          NewOption("vPC Leaf1 to Host1"),
		ID:             "po10",
		VPCConvergence: AdminStDisable,
		Layer:          Layer2,
		MTU:            DefaultMTU,
		Medium:         MediumBroadcast,
		Mode:           SwitchportModeTrunk,
		PcMode:         PortChannelModeActive,
		NativeVlan:     DefaultVLAN,
		SuspIndividual: AdminStEnable,
		TrunkVlans:     "10",
		UserCfgdFlags:  UserFlagAdminState,
	}
	pc.RsmbrIfsItems.RsMbrIfsList.Set(NewPortChannelMember("eth1/10"))
	Register("pc", pc)

	Register("pc_rtd", &PortChannel{
		AccessVlan:     "unknown",
		AdminSt:        AdminStUp,
		Descr:          NewOption("L3 Port-Channel to Spine1"),
		ID:             "po20",
		VPCConvergence: AdminStDisable,
		Layer:          Layer3,
		MTU:            9216,
		Medium:         MediumPointToPoint,
		Mode:           SwitchportModeAccess,
		NativeVlan:     "unknown",
		PcMode:         PortChannelModeActive,
		SuspIndividual: AdminStEnable,
		TrunkVlans:     DefaultVLANRange,
		UserCfgdFlags:  UserFlagAdminState | UserFlagAdminLayer | UserFlagAdminMTU,
		RtvrfMbrItems:  NewVrfMember("po20", "default"),
		AggrExtdItems: struct {
			BufferBoost AdminSt4 `json:"bufferBoost,omitempty"`
		}{BufferBoost: AdminStEnable},
	})

	pcLacp := &PortChannel{
		AccessVlan:     DefaultVLAN,
		AdminSt:        AdminStUp,
		Descr:          NewOption("vPC Leaf1 to Host1 (LACP)"),
		ID:             "po1",
		VPCConvergence: AdminStEnable,
		Layer:          Layer2,
		MTU:            DefaultMTU,
		Medium:         MediumBroadcast,
		Mode:           SwitchportModeTrunk,
		PcMode:         PortChannelModeActive,
		NativeVlan:     DefaultVLAN,
		SuspIndividual: AdminStDisable,
		TrunkVlans:     "10",
		UserCfgdFlags:  UserFlagAdminState,
	}
	pcLacp.RsmbrIfsItems.RsMbrIfsList.Set(NewPortChannelMember("eth1/1"))
	Register("pc_lacp", pcLacp)

	svi := &SwitchVirtualInterface{
		AdminSt: AdminStUp,
		Descr:   "Foo",
		ID:      "vlan10",
		Medium:  SVIMediumBroadcast,
		MTU:     1500,
		VlanID:  10,
	}
	Register("svi", svi)

	fwif := &FabricFwdIf{
		AdminSt: AdminStEnabled,
		ID:      "vlan10",
		Mode:    FwdModeAnycastGateway,
	}
	Register("fwif", fwif)

	dci := &MultisiteIfTracking{IfName: "eth1/1", Tracking: MultisiteIfTrackingModeDCI}
	Register("bgw_tracking", dci)

	bfd := &BFD{AdminSt: AdminStEnabled, ID: "eth1/1"}
	bfd.IfkaItems.DetectMult = 15
	bfd.IfkaItems.MinRxIntvlMs = 100
	bfd.IfkaItems.MinTxIntvlMs = 150
	Register("bfd", bfd)

	icmp := &ICMPIf{ID: "eth1/1", Ctrl: "port-unreachable"}
	Register("rdr", icmp)
}
