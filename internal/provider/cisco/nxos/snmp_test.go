// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	host := &SNMPHost{
		HostName:  "foo.bar",
		UDPPortID: 162,
		CommName:  NewOption("snmpcollector"),
		SecLevel:  "noauth",
		NotifType: "traps",
		Version:   "v2c",
	}
	vrf := &SNMPHostVrf{Vrfname: ManagementVRFName}
	host.UsevrfItems.UseVrfList.Set(vrf)

	hosts := &SNMPHostItems{}
	hosts.HostList.Set(host)
	Register("snmp_host", hosts)

	comm := &SNMPCommunity{Name: "snmpcollector", CommAccess: "unspecified", GrpName: "network-operator"}
	comm.ACLItems.UseACLName = "TEST-ACL"
	items := &SNMPCommunityItems{}
	items.CommSecPList.Set(comm)
	Register("snmp_comm", items)

	srcIf := &SNMPSrcIf{Type: Traps, Ifname: NewOption("mgmt0")}
	Register("snmp_srcif", srcIf)

	info := &SNMPSysInfo{SysContact: NewOption("johndoe@example.com"), SysLocation: NewOption("rack123")}
	Register("snmp_sysinfo", info)

	user := &SNMPUser{Username: "admin", Ipv4AclName: NewOption("TEST-ACL")}
	Register("snmp_user", user)

	traps := &SNMPTrapsItems{}
	traps.CfsItems.StatechangenotifItems = &SNMPTraps{Trapstatus: AdminStEnable}
	Register("snmp_traps", traps)
}
