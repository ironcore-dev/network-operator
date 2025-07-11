// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package logging

// Run the following command to manually extract the list of facilities from a device running NXOS (10.4.3),
// > gnmic --skip-verify get --path System/logging-items | jq -r '.[] | .updates[].values["System/logging-items"]["loglevel-items"]["facility-items"]["Facility-list"][] | "\"\(.facilityName)\","'
//
// Some of the facilities extracted with the command above show a strange behaviour, e.g., `bloggerd“. When attempting to set the logging level
// for them, via CLI or gNMI, the device does *in both cases* neither send an error back or set the logging level to the specified value.
// In fact, `sh run` does not show the specified facility at all after the command is executed. However, if we don't include the facility in the
// list of available facilities the device will send an error back. This occurs with the current diff update implementation.
//
//   - While our client seems to generate updates for the following facilities, these updates have no effect on the device configuration:
//     "bloggerd", "device_test", "ethdstats", "im", "ipqos", "licmgr", "port-channel", "security", "spm", "track"
//
//   - The device includes some facilities that are not present in the list of available facilities:
//     "eth_dstats", "eth_port_channel", "ifmgr", "ipqosmgr", "otm", "securityd"
//
// The following includes  a list of available facilities that MUST be present to avoid errors when setting the logging level.
var AvailableFacilities = []string{
	"aaa",
	"acllog",
	"aclmgr",
	"adbm",
	"adjmgr",
	"arp",
	"ascii-cfg",
	"auth",
	"authpriv",
	"bloggerd",
	"bootvar",
	"callhome",
	"capability",
	"cdp",
	"cert_enroll",
	"cfs",
	"clis",
	"clk_mgr",
	"confcheck",
	"copp",
	"core-dmon",
	"cron",
	"daemon",
	"device_test",
	"dhclient",
	"diag_port_lb",
	"diagclient",
	"diagmgr",
	"eltm",
	"ethdstats",
	"ethpm",
	"evmc",
	"evms",
	"feature-mgr",
	"fs-daemon",
	"ftp",
	"gpixm",
	"icam",
	"icmpv6",
	"igmp",
	"im",
	"ipfib",
	"ipqos",
	"kernel",
	"l2fm",
	"l2pt",
	"l3vm",
	"licmgr",
	"lim",
	"local0",
	"local1",
	"local2",
	"local3",
	"local4",
	"local5",
	"local6",
	"local7",
	"lpr",
	"m2rib",
	"m6rib",
	"mail",
	"mfdm",
	"mmode",
	"module",
	"monitor",
	"mrib",
	"mvsh",
	"netstack",
	"news",
	"ntp",
	"pfstat",
	"pixm",
	"pktmgr",
	"platform",
	"plcmgr",
	"pltfm_config",
	"plugin",
	"port-channel",
	"radius",
	"res_mgr",
	"rpm",
	"sal",
	"security",
	"session-mgr",
	"sksd",
	"smm",
	"snmpd",
	"snmpmib_proc",
	"spanning-tree",
	"spm",
	"stripcl",
	"syslog",
	"sysmgr",
	"track",
	"u6rib",
	"ufdm",
	"urib",
	"user",
	"uucp",
	"vdc_mgr",
	"virtual-service",
	"vlan_mgr",
	"vmm",
	"vshd",
	"xbar",
	"xmlma",
}

// facilities that are only created and can't be updated
var createOnlyFacilities = map[string]struct{}{
	"eth_dstats": {},
	"ifmgr":      {},
	"ipqosmgr":   {},
	"securityd":  {},
}

// these facilities aren't listed but we attempt to modify them via the diff causing
// the device to return an error
var invalidFacilities = map[string]struct{}{
	"eth_port_channel": {},
	"otm":              {},
}
