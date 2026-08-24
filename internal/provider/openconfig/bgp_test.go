// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestAsnToUint32(t *testing.T) {
	tests := []struct {
		name string
		asn  intstr.IntOrString
		want uint32
	}{
		{"int value", intstr.FromInt32(65000), 65000},
		{"string plain", intstr.FromString("4294967295"), 4294967295},
		{"string dotted", intstr.FromString("1.1"), 65537},
		{"string dotted large", intstr.FromString("65535.65535"), 4294967295},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := asnToUint32(test.asn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Errorf("asnToUint32(%v) = %d, want %d", test.asn, got, test.want)
			}
		})
	}
}
