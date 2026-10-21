// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package provider

import "testing"

func TestNotImplementedError(t *testing.T) {
	err := NotImplementedError[InterfaceProvider]{ProviderName: "srlinux.nokia.networking.metal.ironcore.dev"}
	want := `provider "srlinux.nokia.networking.metal.ironcore.dev" does not implement provider.InterfaceProvider`
	if got := err.Error(); got != want {
		t.Errorf("NotImplementedError.Error() = %q, want %q", got, want)
	}
}
