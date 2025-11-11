// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*VLANSystem)(nil)
	_ gnmiext.Defaultable  = (*VLANSystem)(nil)
	_ gnmiext.Configurable = (*VLANReservation)(nil)
	_ gnmiext.Defaultable  = (*VLANReservation)(nil)
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
	AccEncap string `json:"accEncap,omitempty"`
	AdminSt  string `json:"adminSt"` // seems to be always "active" and not changed
	FabEncap string `json:"fabEncap"`
	Name     string `json:"name,omitempty"`
}

func (*VLAN) IsListItem() {}

func (v *VLAN) XPath() string {
	return "System/bd-items/bd-items/BD-list[fabEncap=" + v.FabEncap + "]"
}

func (v *VLAN) SetID(id int) {
	v.FabEncap = "vlan-" + strconv.Itoa(id)
}

func (v *VLAN) SetVNI(id int) {
	v.AccEncap = "vxlan-" + strconv.Itoa(id)
}
