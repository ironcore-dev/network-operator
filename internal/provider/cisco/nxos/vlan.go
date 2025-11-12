// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*VLANSystem)(nil)
	_ gnmiext.Defaultable  = (*VLANSystem)(nil)
	_ gnmiext.Configurable = (*VLANReservation)(nil)
	_ gnmiext.Defaultable  = (*VLANReservation)(nil)
	_ gnmiext.Configurable = (*VLAN)(nil)
	_ gnmiext.Configurable = (*VLANOperItems)(nil)
)

// VLANSystem represents the settings shared among all VLANs
type VLANSystem struct {
	LongName bool `json:"longName"`
}

func (*VLANSystem) XPath() string {
	return "System/vlanmgr-items/inst-items"
}

func (v *VLANSystem) Default() {
	v.LongName = false
}

// VLANReservation represents the settings for VLAN reservations
type VLANReservation struct {
	BlockVal64 bool  `json:"blockVal64"`
	SysVlan    int16 `json:"sysVlan"`
}

func (*VLANReservation) XPath() string {
	return "System/bd-items/resvlan-items"
}

func (v *VLANReservation) Default() {
	v.BlockVal64 = false
	v.SysVlan = 3968 // 4096 - 128
}

// VLAN represents a VLAN configuration on the device
type VLAN struct {
	AccEncap string         `json:"accEncap,omitempty"`
	AdminSt  BdState        `json:"adminSt"`
	BdState  BdState        `json:"BdState"` // Note the capitalization of this fields JSON tag
	FabEncap string         `json:"fabEncap"`
	Name     Option[string] `json:"name"`
}

func (*VLAN) IsListItem() {}

func (v *VLAN) XPath() string {
	return "System/bd-items/bd-items/BD-list[fabEncap=" + v.FabEncap + "]"
}

type VLANOperItems struct {
	FabEncap string `json:"-"`
	OperSt   OperSt `json:"operSt"`
}

func (*VLANOperItems) IsListItem() {}

func (v *VLANOperItems) XPath() string {
	return "System/bd-items/bd-items/BD-list[fabEncap=" + v.FabEncap + "]"
}

type BdState string

const (
	// BdStateActive indicates that the bridge domain is active
	BdStateActive BdState = "active"
	// BdStateInactive indicates that the bridge domain is inactive/suspended
	BdStateInactive BdState = "suspend"
)
