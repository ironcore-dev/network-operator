// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*Feature)(nil)
	_ gnmiext.Defaultable  = (*Feature)(nil)
)

// Feature represents a dynamic feature configuration on a NX-OS device.
type Feature struct {
	Name    string  `json:"-"`
	AdminSt AdminSt `json:"adminSt"`
}

func (f *Feature) XPath() string {
	return "System/fm-items/" + f.Name + "-items"
}

func (f *Feature) Default() {
	f.AdminSt = AdminStDisabled
}
