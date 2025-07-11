// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package feat

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Features(t *testing.T) {
	f := Features{"restconf"}

	got, err := f.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/fm-items" {
		t.Errorf("expected key 'System/fm-items' to be present")
	}

	fm, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems")
	}

	if fm.RestconfItems == nil || fm.RestconfItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected value for 'System/fm-items/restconf-items/adminSt' to be enabled, got %v", fm.RestconfItems.AdminSt)
	}
}
