// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	v := &VPCIf{ID: 10}
	v.SetPortChannel("po10")
	Register("vpc_member", v)

	vd := &VPCDomain{
		AdminSt:                 AdminStEnabled,
		AutoRecovery:            NewOption(AdminStEnabled),
		AutoRecoveryReloadDelay: 360,
		DelayRestoreSVI:         NewOption[uint16](45),
		DelayRestoreVPC:         NewOption[uint16](150),
		FastConvergence:         NewOption(AdminStEnabled),
		Id:                      2,
		L3PeerRouter:            AdminStEnabled,
		PeerGateway:             AdminStEnabled,
		PeerSwitch:              AdminStEnabled,
		RolePrio:                NewOption[uint16](100),
		SysPrio:                 NewOption[uint16](10),
	}
	vd.KeepAliveItems.DestIP = "10.114.235.156"
	vd.KeepAliveItems.SrcIP = "10.114.235.155"
	vd.KeepAliveItems.VRF = "management"
	Register("vpcdomain", vd)
}
