// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

// Version represents the operating system version of the target device.
type Version string

const (
	VersionUnknown  Version = "Unknown"
	VersionNX10_4_3 Version = "10.4(3)"
	VersionNX10_4_4 Version = "10.4(4)"
	VersionNX10_4_5 Version = "10.4(5)"
	VersionNX10_4_6 Version = "10.4(6)"
	VersionNX10_5_1 Version = "10.5(1)"
	VersionNX10_5_2 Version = "10.5(2)"
	VersionNX10_5_3 Version = "10.5(3)"
	VersionNX10_6_1 Version = "10.6(1)"
)

// nxosVersions maps the revision date of the Cisco-NX-OS-device yang model to the corresponding [Version].
// It is used to determine the version of the target device based on the capabilities returned by the device.
var nxosVersions = map[string]Version{
	"2024-03-26": VersionNX10_4_3,
	"2024-10-17": VersionNX10_4_4,
	"2025-03-01": VersionNX10_4_5,
	"2025-08-30": VersionNX10_4_6,
	"2024-07-25": VersionNX10_5_1,
	"2024-11-26": VersionNX10_5_2,
	"2025-04-23": VersionNX10_5_3,
	"2025-08-12": VersionNX10_6_1,
}

// NXVersion returns the NX-OS operating system version of the target device based on the supported models.
// If the version cannot be determined, [VersionUnknown] is returned.
func NXVersion(c *gnmiext.Capabilities) Version {
	version := VersionUnknown
	for _, m := range c.SupportedModels {
		if m.Name == "Cisco-NX-OS-device" && m.Organization == "Cisco Systems, Inc." {
			if v, ok := nxosVersions[m.Version]; ok {
				version = v
			}
			break
		}
	}
	return version
}
