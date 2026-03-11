// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestIPPrefix_IsPointToPoint(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   bool
	}{
		{name: "IPv4 /31 is p2p", prefix: "10.0.0.0/31", want: true},
		{name: "IPv4 /32 is not p2p", prefix: "10.0.0.1/32", want: false},
		{name: "IPv4 /30 is not p2p", prefix: "10.0.0.0/30", want: false},
		{name: "IPv4 /24 is not p2p", prefix: "192.168.1.0/24", want: false},
		{name: "IPv6 /127 is p2p", prefix: "2001:db8::/127", want: true},
		{name: "IPv6 /128 is not p2p", prefix: "2001:db8::1/128", want: false},
		{name: "IPv6 /126 is not p2p", prefix: "2001:db8::/126", want: false},
		{name: "IPv6 /64 is not p2p", prefix: "2001:db8::/64", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := MustParsePrefix(tt.prefix)
			if got := p.IsPointToPoint(); got != tt.want {
				t.Errorf("IPPrefix(%q).IsPointToPoint() = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}
