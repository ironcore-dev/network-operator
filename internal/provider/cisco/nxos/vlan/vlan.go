// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package vlan provides functionality to manage VLANs on Cisco NX-OS devices. This implementation assumes that the
//
// [Cisco-VLAN] https://www.cisco.com/c/en/us/td/docs/dcn/nx-os/nexus9000/104x/configuration/layer-2-switching/cisco-nexus-9000-series-nx-os-layer-2-switching-configuration-guide-104x/m-configuring-vlans.html
package vlan

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/openconfig/ygot/ygot"
)

var _ gnmiext.DeviceConf = (*VLAN)(nil)

type VLAN struct {
	ID         uint16
	Name       string
	AdminState bool
}

func (v *VLAN) validate(ctx context.Context, c gnmiext.Client) error {
	if v.ID < 1 || v.ID > 4095 {
		return errors.New("vlan: ID must be in the range 1-4095")
	}
	// always reserved VLAN IDs: 1, 4093, 4094, and 4095
	if v.ID == 1 || v.ID >= 4093 {
		return fmt.Errorf("vlan: ID %d is reserved and cannot be configured", v.ID)
	}
	// the reserved ID range for internal use can be configured by the user and needs to be thus fetched from the device
	rv := &nxos.Cisco_NX_OSDevice_System_BdItems_ResvlanItems{}
	err := c.Get(ctx, "System/bd-items/resvlan-items", rv)
	if err != nil {
		return fmt.Errorf("vlan: failed to retrieve reserved VLAN range from device: %w", err)
	}
	if rv.SysVlan == nil || *rv.SysVlan == 0 {
		return fmt.Errorf("vlan: failed to retrieve reserved VLAN range from device, sysVlan is nil or zero")
	}
	if v.ID >= *rv.SysVlan && v.ID <= *rv.SysVlan+128 {
		return fmt.Errorf("vlan: ID %d is in the range reserved for internal use, min: %d, max %d", v.ID, *rv.SysVlan, *rv.SysVlan+128)
	}

	if v.ID >= 1006 && v.ID <= 3967 && !v.AdminState {
		return errors.New("vlan: IDs 1006-3967 are reserved for extended range and cannot be shut down or deleted")
	}
	return nil
}

// ToYGOT converts the VLAN configuration to a list of gnmiext.Updates that can be applied to the device.
// Returns an error if the VLAN ID is out of range or reserved (see [Cisco-VLAN] for applicable ranges)
func (v *VLAN) ToYGOT(ctx context.Context, c gnmiext.Client) ([]gnmiext.Update, error) {
	if err := v.validate(ctx, c); err != nil {
		return nil, err
	}

	yv := &nxos.Cisco_NX_OSDevice_System_BdItems_BdItems_BDList{
		Id:      ygot.Uint32(uint32(v.ID)),
		AdminSt: nxos.Cisco_NX_OSDevice_L2_DomAdminSt_active,
	}

	if v.Name != "" {
		yv.Name = ygot.String(v.Name)
	}
	if !v.AdminState {
		yv.AdminSt = nxos.Cisco_NX_OSDevice_L2_DomAdminSt_suspend
	}

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-" + strconv.FormatUint(uint64(v.ID), 10) + "]",
			Value: yv,
		},
	}, nil
}

// Reset resets or deletes the VLAN depending on VLAN's ID value (see [Cisco-VLAN] for applicable ranges):
func (v *VLAN) Reset(ctx context.Context, c gnmiext.Client) ([]gnmiext.Update, error) {
	if err := v.validate(ctx, c); err != nil {
		return nil, err
	}
	// normal VLANs can be deleted
	if v.ID >= 2 && v.ID <= 1005 {
		return []gnmiext.Update{
			gnmiext.DeletingUpdate{
				XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-" + strconv.FormatUint(uint64(v.ID), 10) + "]",
			},
		}, nil
	}
	// extended range VLANs can only be reset to default values
	yv := &nxos.Cisco_NX_OSDevice_System_BdItems_BdItems_BDList{}
	yv.PopulateDefaults()
	yv.Id = ygot.Uint32(uint32(v.ID))
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/bd-items/bd-items/BD-list[fabEncap=vlan-" + strconv.FormatUint(uint64(v.ID), 10) + "]",
		},
	}, nil
}
