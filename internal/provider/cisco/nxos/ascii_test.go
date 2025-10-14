// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"
	"testing"
)

func TestASCII(t *testing.T) {
	tests := []struct {
		got  ASCIIStr
		want string
	}{
		{
			got:  "49,48,103,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0",
			want: "10g",
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			if test.got.String() != test.want {
				t.Errorf("ASCIIStr.String() = %v, want %v", test.got.String(), test.want)
			}
		})
	}
}
