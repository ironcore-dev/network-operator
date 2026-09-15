// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "net/netip"

func init() {
	vd := &VPCDomain{
		AdminSt:                 AdminStEnabled,
		AutoRecovery:            AdminStEnabled,
		AutoRecoveryReloadDelay: 360,
		DelayRestoreSVI:         45,
		DelayRestoreVPC:         150,
		FastConvergence:         AdminStEnabled,
		ID:                      2,
		L3PeerRouter:            AdminStEnabled,
		PeerGateway:             AdminStEnabled,
		PeerSwitch:              AdminStEnabled,
		RolePrio:                100,
		SysPrio:                 10,
	}
	vd.KeepAliveItems.DestIP = Prefix(netip.MustParsePrefix("10.114.235.156/32"))
	vd.KeepAliveItems.SrcIP = Prefix(netip.MustParsePrefix("10.114.235.155/32"))
	vd.KeepAliveItems.VRF = ManagementVRFName
	vd.KeepAliveItems.PeerLinkItems.AdminSt = AdminStEnabled
	vd.KeepAliveItems.PeerLinkItems.ID = "po1"
	Register("vpc_domain", vd)

	vi := &VPCIf{ID: 10}
	vi.SetPortChannel("po10")
	Register("vpc_member", vi)
}
