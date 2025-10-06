// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "testing"

func TestShortName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// Valid Ethernet interface names
		{
			name:     "ethernet full name",
			input:    "Ethernet1/1",
			expected: "eth1/1",
			wantErr:  false,
		},
		{
			name:     "ethernet short name",
			input:    "eth1/1",
			expected: "eth1/1",
			wantErr:  false,
		},
		{
			name:     "ethernet with multiple digits",
			input:    "Ethernet10/24",
			expected: "eth10/24",
			wantErr:  false,
		},

		// Valid Loopback interface names
		{
			name:     "loopback full name",
			input:    "Loopback1",
			expected: "lo1",
			wantErr:  false,
		},
		{
			name:     "loopback short name",
			input:    "lo1",
			expected: "lo1",
			wantErr:  false,
		},
		{
			name:     "loopback with multiple digits",
			input:    "Loopback100",
			expected: "lo100",
			wantErr:  false,
		},

		// Error cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "unsupported interface type",
			input:    "Foobar1/1",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid ethernet format - missing slash",
			input:    "Ethernet11",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid loopback format",
			input:    "Loopback1/1",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid format",
			input:    "1/1",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "random string",
			input:    "random",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "port-channel full name",
			input:    "Port-Channel10",
			expected: "po10",
			wantErr:  false,
		},
		{
			name:     "port-channel short name",
			input:    "po10",
			expected: "po10",
			wantErr:  false,
		},
		{
			name:     "port-channel with multiple digits",
			input:    "Port-Channel123",
			expected: "po123",
			wantErr:  false,
		},
		{
			name:     "invalid port-channel format",
			input:    "PortChannel10",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid port-channel format",
			input:    "po100a",
			expected: "",
			wantErr:  true,
		},

		// Management interface names
		{
			name:     "valid management interface",
			input:    "mgmt0",
			expected: "mgmt0",
			wantErr:  false,
		},
		{
			name:     "invalid management interface",
			input:    "mgmt1",
			expected: "",
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ShortName(test.input)
			if test.wantErr {
				if err == nil {
					t.Errorf("ShortName(%q) expected error, but got none", test.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ShortName(%q) unexpected error: %v", test.input, err)
				return
			}
			if result != test.expected {
				t.Errorf("ShortName(%q) = %q, expected %q", test.input, result, test.expected)
			}
		})
	}
}
