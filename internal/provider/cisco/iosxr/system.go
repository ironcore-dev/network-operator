// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

const Manufacturer = "Cisco"

// Model is the chassis model of the device, e.g. "NCS-57C3-MOD-SYS".
// SerialNumber is the serial number of the device, e.g. "9VT9OHZBC3H".
// FirmwareVersion is the firmware version of the device, e.g. "25.2.2".
type BasicDeviceInfo struct {
	Model           string `json:"model-name"`
	SerialNumber    string `json:"serial-number"`
	FirmwareVersion string `json:"firmware-version"`
}

func (*BasicDeviceInfo) XPath() string {
	return "Cisco-IOS-XR-platform-inventory-oper:/platform-inventory/racks/rack[name=0]/attributes/basic-info"
}
