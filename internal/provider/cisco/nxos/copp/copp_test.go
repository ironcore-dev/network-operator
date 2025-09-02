// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package copp

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_COPP(t *testing.T) {
	copp := &COPP{Profile: Moderate}

	got, err := copp.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/copp-items/profile-items" {
		t.Errorf("expected key 'System/copp-items/profile-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_CoppItems_ProfileItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_CoppItems_ProfileItems, got %T", update.Value)
	}
}
