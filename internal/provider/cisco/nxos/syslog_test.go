// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("syslog_origin", &SyslogOrigin{Idtype: "hostname"})
	Register("syslog_history", &SyslogHistory{Level: Informational, Size: 500})
	Register("syslog_srcif", &SyslogSrcIf{AdminSt: AdminStEnabled, IfName: "mgmt0"})

	reItems := new(SyslogRemoteItems)
	reItems.RemoteDestList.Set(&SyslogRemote{
		ForwardingFacility: "local7",
		Host:               "10.10.10.10",
		Port:               514,
		Severity:           Informational,
		Transport:          "udp",
		VrfName:            ManagementVRFName,
	})
	Register("syslog_remote", reItems)

	facItems := new(SyslogFacilityItems)
	facItems.FacilityList.Set(&SyslogFacility{FacilityName: "aaa", SeverityLevel: Informational})
	Register("syslog_facility", facItems)
}
