// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package testutils

import (
	"testing"

	"github.com/openconfig/ygot/ygot"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func AssertEqual(t *testing.T, a, b []gnmiext.Update) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("different number of updates: %d != %d", len(a), len(b))
	}
	for i, u := range a {
		switch u.(type) {
		case gnmiext.DeletingUpdate:
			if _, ok := b[i].(gnmiext.DeletingUpdate); !ok {
				t.Errorf("expected DeletingUpdate at index %d, got %T", i, b[i])
				continue
			}
			assertEqualXPath(t, a[i].(gnmiext.DeletingUpdate).XPath, b[i].(gnmiext.DeletingUpdate).XPath, i)
		case gnmiext.EditingUpdate:
			if _, ok := b[i].(gnmiext.EditingUpdate); !ok {
				t.Errorf("expected EditingUpdate at index %d, got %T", i, b[i])
				continue
			}
			assertEqualXPath(t, a[i].(gnmiext.EditingUpdate).XPath, b[i].(gnmiext.EditingUpdate).XPath, i)
			assertEqualYGot(t, a[i].(gnmiext.EditingUpdate).Value, b[i].(gnmiext.EditingUpdate).Value)
		case gnmiext.ReplacingUpdate:
			if _, ok := b[i].(gnmiext.ReplacingUpdate); !ok {
				t.Errorf("expected ReplacingUpdate at index %d, got %T", i, b[i])
				continue
			}
			assertEqualXPath(t, a[i].(gnmiext.ReplacingUpdate).XPath, b[i].(gnmiext.ReplacingUpdate).XPath, i)
			assertEqualYGot(t, a[i].(gnmiext.ReplacingUpdate).Value, b[i].(gnmiext.ReplacingUpdate).Value)
		default:
			t.Errorf("unexpected update type at index %d: %T", i, u)
			continue
		}
	}
}

func assertEqualXPath(t *testing.T, a, b string, i int) {
	t.Helper()
	if a != b {
		t.Errorf("different xpath at index %d: %s != %s", i, a, b)
	}
}

func assertEqualYGot(t *testing.T, a, b ygot.GoStruct) {
	t.Helper()
	notification, err := ygot.Diff(a, b)
	if err != nil {
		t.Errorf("failed to compute diff: %v", err)
		return
	}
	if len(notification.Update) > 0 || len(notification.Delete) > 0 {
		t.Errorf("unexpected diff: %s", notification.String())
	}
}
