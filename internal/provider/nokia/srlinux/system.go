// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

const Manufacturer = "Nokia"

var (
	_ gnmiext.DataElement = (*Name)(nil)
	_ gnmiext.DataElement = (*ChassisState)(nil)
	_ gnmiext.DataElement = (*SystemInformation)(nil)
)

// Name maps the SR Linux system name container, which carries the device host-name.
type Name struct {
	HostName string `json:"host-name"`
}

func (*Name) XPath() string {
	return "srl_nokia-system:system/srl_nokia-system-name:name"
}

// ChassisState maps the SR Linux platform chassis state.
type ChassisState struct {
	// Type is the chassis type, used as the device model (e.g. "7220 IXR-D3L").
	Type         string `json:"type,omitempty"`
	SerialNumber string `json:"serial-number"`
}

func (*ChassisState) XPath() string {
	return "srl_nokia-platform:platform/srl_nokia-platform-chassis:chassis"
}

// SystemInformation maps the SR Linux system information state. It carries both the running
// version and the last boot time so GetDeviceInfo and GetLastRebootTime can share it.
type SystemInformation struct {
	Version    string `json:"version"`
	LastBooted string `json:"last-booted"`
}

func (*SystemInformation) XPath() string {
	return "srl_nokia-system:system/srl_nokia-system-info:information"
}
