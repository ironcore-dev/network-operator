// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("esi_interface", &EthernetSegmentItems{
		ID:   "po10",
		Type: EthernetSegmentTypeNative,
		ESI:  NewOption("0011.2233.4455.6677.8801"),
	})

	Register("esi_interface_sysmac", &EthernetSegmentItems{
		ID:              "po10",
		Type:            EthernetSegmentTypeNative,
		SysMac:          NewOption("00:11:22:33:44:55"),
		LocalIdentifier: 1,
	})

	Register("esi_multihoming", &MultihomingItems{
		EadEviRoute:    true,
		AdminSt:        AdminStEnabled,
		DfElectionMode: DfElectModeModulo,
		DfElectionTime: "2.2",
	})

	Register("esi_evpn_multicast", &EvpnMulticastItems{
		State: AdminStEnabled,
	})
}
