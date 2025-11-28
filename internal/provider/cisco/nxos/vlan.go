// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*VLANSystem)(nil)
	_ gnmiext.Defaultable  = (*VLANSystem)(nil)
	_ gnmiext.Configurable = (*VLANReservation)(nil)
	_ gnmiext.Defaultable  = (*VLANReservation)(nil)
	_ gnmiext.Configurable = (*VLAN)(nil)
	_ gnmiext.Configurable = (*VLANOperItems)(nil)
	_ gnmiext.Configurable = (*VXLAN)(nil)
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

type BDItems struct {
	BdList []struct {
		AccEncap string `json:"accEncap"`
		FabEncap string `json:"fabEncap"`
	} `json:"BD-list"`
}

func (*BDItems) XPath() string {
	return "System/bd-items/bd-items"
}

func (b *BDItems) GetByVXLAN(v string) *VXLAN {
	for _, bd := range b.BdList {
		if bd.AccEncap == v {
			return &VXLAN{
				AccEncap: bd.AccEncap,
				FabEncap: bd.FabEncap,
			}
		}
	}
	return nil
}

var (
	_ json.Marshaler   = VXLAN{}
	_ json.Unmarshaler = (*VXLAN)(nil)
)

// VXLAN represents VXLAN encapsulation settings for a VLAN.
// It is part of the Bridge Domain configuration of a VLAN.
type VXLAN struct {
	AccEncap string `json:"-"`
	FabEncap string `json:"-"`
}

func (v *VXLAN) XPath() string {
	return "System/bd-items/bd-items/BD-list[fabEncap=" + v.FabEncap + "]/accEncap"
}

func (v VXLAN) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.AccEncap)
}

func (v *VXLAN) UnmarshalJSON(b []byte) error {
	var encap string
	if err := json.Unmarshal(b, &encap); err != nil {
		return err
	}
	v.AccEncap = encap
	return nil
}
