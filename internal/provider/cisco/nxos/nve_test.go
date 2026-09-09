// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	nve := &NVE{
		AdminSt:          AdminStEnabled,
		HostReach:        HostReachBGP,
		AdvertiseVmac:    true,
		SourceInterface:  "lo0",
		AnycastInterface: NewOption("lo1"),
		SuppressARP:      true,
		McastGroupL2:     NewOption("237.0.0.1"),
		McastGroupL3:     NewOption(""),
		HoldDownTime:     300,
	}
	Register("nve", nve)

	vni := &VNI{
		Vni:        100010,
		McastGroup: NewOption("239.1.1.100"),
	}
	Register("vni", vni)

	suppressARPTrue := true
	vniSuppressARPTrue := &VNI{
		Vni:         100011,
		SuppressARP: Option[bool]{Value: &suppressARPTrue},
	}
	Register("vni_suppress_arp_true", vniSuppressARPTrue)

	suppressARPFalse := false
	vniSuppressARPFalse := &VNI{
		Vni:         100012,
		SuppressARP: Option[bool]{Value: &suppressARPFalse},
	}
	Register("vni_suppress_arp_false", vniSuppressARPFalse)
	nveInfraVLANs := &NVEInfraVLANs{
		InfraVLANList: []*NVEInfraVLAN{
			{ID: 4052},
			{ID: 4092},
		},
	}
	Register("infra_vlans", nveInfraVLANs)

	ffw := &FabricFwd{
		AdminSt: "enabled",
		Address: "00:00:11:11:22:22",
	}
	Register("fabric_forward", ffw)
}
