// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Trustpoint(t *testing.T) {
	tp := &Trustpoint{ID: "mytrustpoint"}

	// Mock the client to return false for Exists (trustpoint doesn't exist)
	mockClient := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			expectedXPath := "System/userext-items/pkiext-items/tp-items/TP-list[name=mytrustpoint]"
			if xpath != expectedXPath {
				t.Errorf("Exists called with unexpected xpath: got=%s, want=%s", xpath, expectedXPath)
			}
			return false, nil
		},
	}

	got, err := tp.ToYGOT(t.Context(), mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}

	if update.XPath != "System/userext-items/pkiext-items/tp-items/TP-list[name=mytrustpoint]" {
		t.Errorf("expected key 'System/userext-items/pkiext-items/tp-items/TP-list[name=mytrustpoint]' to be present")
	}

	ti, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList")
	}

	// Create expected struct with PopulateDefaults to match the implementation
	want := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList{}
	want.PopulateDefaults()
	want.Name = ygot.String("mytrustpoint")

	if !reflect.DeepEqual(ti, want) {
		t.Errorf("unexpected value for 'System/userext-items/pkiext-items/tp-items/TP-list[name=mytrustpoint]': got=%+v, want=%+v", ti, want)
	}

	// Verify that Exists was called exactly once
	existsCalls := mockClient.ExistsCalls()
	if len(existsCalls) != 1 {
		t.Errorf("expected Exists to be called once, got %d calls", len(existsCalls))
	}
}

func Test_Trustpoint_AlreadyExists(t *testing.T) {
	tp := &Trustpoint{ID: "mytrustpoint"}

	// Mock the client to return true for Exists (trustpoint already exists)
	mockClient := &gnmiext.ClientMock{
		ExistsFunc: func(ctx context.Context, xpath string) (bool, error) {
			expectedXPath := "System/userext-items/pkiext-items/tp-items/TP-list[name=mytrustpoint]"
			if xpath != expectedXPath {
				t.Errorf("Exists called with unexpected xpath: got=%s, want=%s", xpath, expectedXPath)
			}
			return true, nil
		},
	}

	_, err := tp.ToYGOT(t.Context(), mockClient)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	// Verify that Exists was called exactly once
	existsCalls := mockClient.ExistsCalls()
	if len(existsCalls) != 1 {
		t.Errorf("expected Exists to be called once, got %d calls", len(existsCalls))
	}
}
