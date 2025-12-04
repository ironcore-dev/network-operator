// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	v := &VPCIf{ID: 10}
	v.SetPortChannel("po10")
	Register("vpc_member", v)

	vd := &VPCDomain{
		AdminSt:                 AdminStEnabled,
		AutoRecovery:            AdminStEnabled,
		AutoRecoveryReloadDelay: 360,
		DelayRestoreSVI:         45,
		DelayRestoreVPC:         150,
		FastConvergence:         AdminStEnabled,
		Id:                      2,
		L3PeerRouter:            AdminStEnabled,
		PeerGateway:             AdminStEnabled,
		PeerSwitch:              AdminStEnabled,
		RolePrio:                100,
		SysPrio:                 10,
	}
	vd.KeepAliveItems.DestIP = "10.114.235.156"
	vd.KeepAliveItems.SrcIP = "10.114.235.155"
	vd.KeepAliveItems.VRF = "management"
	Register("vpcdomain", vd)
}
