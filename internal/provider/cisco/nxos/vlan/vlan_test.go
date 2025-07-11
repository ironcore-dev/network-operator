// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package vlan

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_VLAN(t *testing.T) {
	vlan := &VLAN{
		LongName: true,
	}

	got, err := vlan.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/vlanmgr-items/inst-items" {
		t.Errorf("expected key 'System/vlanmgr-items/inst-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_VlanmgrItems_InstItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_VlanmgrItems_InstItems")
	}
}
