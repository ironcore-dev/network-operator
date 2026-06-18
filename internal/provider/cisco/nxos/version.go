// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/transport/gnmiext"

// Version represents the operating system version of the target device.
// Versions are ordered so that comparison operators (>, >=, <, <=) reflect
// the actual release ordering.
type Version uint8

const (
	VersionUnknown  Version = iota
	VersionNX10_4_3         // 10.4(3)
	VersionNX10_4_4         // 10.4(4)
	VersionNX10_4_5         // 10.4(5)
	VersionNX10_4_6         // 10.4(6)
	VersionNX10_5_1         // 10.5(1)
	VersionNX10_5_2         // 10.5(2)
	VersionNX10_5_3         // 10.5(3)
	VersionNX10_6_1         // 10.6(1)
	VersionNX10_6_2         // 10.6(2)
	VersionNX10_6_3         // 10.6(3)

	VersionNX10_7_1 // 10.7(1)
)

func (v Version) String() string {
	switch v {
	case VersionNX10_4_3:
		return "10.4(3)"
	case VersionNX10_4_4:
		return "10.4(4)"
	case VersionNX10_4_5:
		return "10.4(5)"
	case VersionNX10_4_6:
		return "10.4(6)"
	case VersionNX10_5_1:
		return "10.5(1)"
	case VersionNX10_5_2:
		return "10.5(2)"
	case VersionNX10_5_3:
		return "10.5(3)"
	case VersionNX10_6_1:
		return "10.6(1)"
	case VersionNX10_6_2:
		return "10.6(2)"
	case VersionNX10_6_3:
		return "10.6(3)"
	case VersionNX10_7_1:
		return "10.7(1)"
	default:
		return "Unknown"
	}
}

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
	"2025-12-12": VersionNX10_6_2,
	"2026-04-24": VersionNX10_6_3,

	// VersionNX10_7_1 is the minimum version that supports CA chains via gNOI LoadCertificate.
	// TODO: Update with the correct YANG model revision date once NX-OS 10.7(1) is released.
	"9999-01-01": VersionNX10_7_1,
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
