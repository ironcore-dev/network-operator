// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

const Manufacturer = "Cisco"

var (
	_ gnmiext.Configurable = (*SystemJumboMTU)(nil)
	_ gnmiext.Defaultable  = (*SystemJumboMTU)(nil)
	_ gnmiext.Configurable = (*Model)(nil)
	_ gnmiext.Configurable = (*SerialNumber)(nil)
	_ gnmiext.Configurable = (*FirmwareVersion)(nil)
)

// System represents general system settings.
type SystemJumboMTU int16

func (s *SystemJumboMTU) XPath() string {
	return "System/ethpm-items/inst-items/systemJumboMtu"
}

func (s *SystemJumboMTU) Default() {
	*s = 9216
}

// Model is the chassis model of the device, e.g. "N9K-C9336C-FX2".
type Model string

func (*Model) XPath() string {
	return "System/ch-items/model"
}

// SerialNumber is the serial number of the device, e.g. "9VT9OHZBC3H".
// This value should typically match the serial number
// of the chassis under "System/ch-items/ser".
type SerialNumber string

func (*SerialNumber) XPath() string {
	return "System/serial"
}

// FirmwareVersion is the firmware version of the device, e.g. "10.4(3)".
type FirmwareVersion string

func (*FirmwareVersion) XPath() string {
	return "System/showversion-items/nxosVersion"
}
