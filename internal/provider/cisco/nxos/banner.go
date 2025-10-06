// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*Banner)(nil)
	_ gnmiext.Defaultable  = (*Banner)(nil)
)

// Banner represents the pre-login banner configuration of the device.
type Banner struct {
	// The value of the delimiter used to start and end the banner message
	Delimiter string `json:"delimiter"`
	// String to be displayed as the banner message
	Message string `json:"message"`
}

func (*Banner) XPath() string {
	return "System/userext-items/preloginbanner-items"
}

func (b *Banner) Default() {
	b.Delimiter = "#"
	b.Message = "User Access Verification\n"
}
