// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package vlan

import (
	"context"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*VLAN)(nil)

type VLAN struct {
	// If configured as "true" then long strings will be allowed when naming VLANs
	LongName bool
}

func (n *VLAN) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/vlanmgr-items/inst-items",
			Value: &nxos.Cisco_NX_OSDevice_System_VlanmgrItems_InstItems{LongName: ygot.Bool(n.LongName)},
		},
	}, nil
}

func (v *VLAN) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	vlan := &nxos.Cisco_NX_OSDevice_System_VlanmgrItems_InstItems{}
	vlan.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/vlanmgr-items/inst-items",
			Value: vlan,
		},
	}, nil
}
