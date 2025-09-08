// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package vlan

import (
	"context"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Settings)(nil)

// Settings represents the settings shared among all VLANs
type Settings struct {
	// If configured as "true" then long strings will be allowed when naming VLANs
	LongName bool
}

func (s *Settings) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/vlanmgr-items/inst-items",
			Value: &nxos.Cisco_NX_OSDevice_System_VlanmgrItems_InstItems{LongName: ygot.Bool(s.LongName)},
		},
	}, nil
}

func (s *Settings) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/vlanmgr-items/inst-items",
		},
	}, nil
}
