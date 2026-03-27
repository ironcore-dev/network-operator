// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iosxr

func init() {
	dInfo := BasicDeviceInfo{
		Model:           "NCS-57C3-MOD-SYS",
		SerialNumber:    "9VT9OHZBC3H",
		FirmwareVersion: "25.2.2",
	}
	Register("system", &dInfo)

	sysTime := SystemTime{
		CurrTime: Clock{
			Day:         1,
			Hour:        1,
			Millisecond: 1,
			Minute:      1,
			Month:       1,
			Second:      1,
			TimeZone:    "UTC",
			Year:        1990,
		},
		Uptime: Uptime{
			Uptime: 123456,
		},
	}
	Register("system_time", &sysTime)
}
