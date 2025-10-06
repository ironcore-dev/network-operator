// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*CoPP)(nil)
	_ gnmiext.Defaultable  = (*CoPP)(nil)
)

// CoPP represents the Control Plane Policing profile configuration on a NX-OS device.
type CoPP struct {
	Profile Profile `json:"prof"`
}

func (*CoPP) XPath() string {
	return "System/copp-items/profile-items"
}

func (c *CoPP) Default() {
	c.Profile = Strict
}

// Profile represents a CoPP profile.
type Profile string

const (
	Strict   Profile = "strict"
	Moderate Profile = "moderate"
	Dense    Profile = "dense"
	Lenient  Profile = "lenient"
)
