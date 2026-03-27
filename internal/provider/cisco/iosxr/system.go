// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import "time"

const Manufacturer = "Cisco"

type BasicDeviceInfo struct {
	// Model is the chassis model of the device, e.g. "NCS-57C3-MOD-SYS".
	Model string `json:"model-name"`

	// SerialNumber is the serial number of the device, e.g. "9VT9OHZBC3H".
	SerialNumber string `json:"serial-number"`

	// FirmwareVersion is the firmware version of the device, e.g. "25.2.2".
	FirmwareVersion string `json:"firmware-version"`

	// Hostname is the hostname of the device, e.g. "router1".
	Hostname string `json:"name"`
}

func (*BasicDeviceInfo) XPath() string {
	// Rack name defines the physical chassis, which is 0 for single-chassis devices.
	return "Cisco-IOS-XR-platform-inventory-oper:/platform-inventory/racks/rack[name=0]/attributes/basic-info"
}

type SystemTime struct {
	CurrTime Clock  `json:"clock"`
	Uptime   Uptime `json:"uptime"`
}

type Uptime struct {
	// Uptime stores the uptime in seconds
	Uptime uint32 `json:"uptime"`
}

type Clock struct {
	Day         int    `json:"day"`
	Hour        int    `json:"hour"`
	Millisecond int    `json:"millisecond"`
	Minute      int    `json:"minute"`
	Month       int    `json:"month"`
	Second      int    `json:"second"`
	TimeZone    string `json:"time-zone"`
	Year        int    `json:"year"`
}

func (*SystemTime) XPath() string {
	return "Cisco-IOS-XR-shellutil-oper:/system-time"
}

func (c *Clock) ConvertToTime() time.Time {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	return time.Date(c.Year, time.Month(c.Month), c.Day, c.Hour, c.Minute, c.Second, c.Millisecond, loc)
}
