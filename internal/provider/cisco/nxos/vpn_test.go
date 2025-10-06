// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"testing"
)

func TestNewVPNIPv4Address(t *testing.T) {
	tests := []struct {
		afType  AFTYPE
		value   string
		wantErr bool
	}{
		// Type 0 valid
		{AFType0, "1:1", false},          // minimal valid ASN and assigned number
		{AFType0, "65534:0", false},      // max valid ASN and assigned number
		{AFType0, "1:4294967295", false}, // minimal valid ASN and assigned number
		{AFType0, "1:4294967295", false}, // max valid ASN and assigned number
		// Type 1 valid
		{AFType1, "10.0.0.1:1", false},        // valid IPv4, min assigned number
		{AFType1, "192.168.1.1:65535", false}, // valid IPv4, max assigned number
		// Type 2 valid
		{AFType2, "1:0", false},              // minimal valid ASN and assigned number
		{AFType2, "4294967294:65535", false}, // max valid ASN and assigned number
		// Type 0 invalid
		{AFType0, "", true},            // empty
		{AFType0, ":", true},           // missing values
		{AFType0, ":100", true},        // missing ASN
		{AFType0, "100:", true},        // missing assigned number
		{AFType0, "abc:100", true},     // ASN not numeric
		{AFType0, "1:abc", true},       // assigned number not numeric
		{AFType0, "0:100", true},       // ASN 0 reserved
		{AFType0, "65535:1000", true},  // assigned number
		{AFType0, "-1:100", true},      // negative ASN
		{AFType0, "100:-100", true},    // negative assigned number
		{AFType0, "001:100", true},     // leading zero in ASN
		{AFType0, "1:001", true},       // leading zero in assigned number
		{AFType0, "65000:", true},      // missing assigned number
		{AFType0, ":100", true},        // missing ASN
		{AFType0, "65000:abc", true},   // assigned number not numeric
		{AFType0, "70000:12345", true}, // ASN too large for type 0
		// Type 1 invalid
		{AFType1, "", true},                   // empty
		{AFType1, ":", true},                  // missing values
		{AFType1, "1.2.3.4:", true},           // missing assigned number
		{AFType1, ":100", true},               // missing IP
		{AFType1, "abc:100", true},            // not an IP
		{AFType1, "256.1.1.1:100", true},      // invalid IP octet
		{AFType1, "1.2.3.4.5:100", true},      // too many octets
		{AFType1, "192.0.100.1/32:100", true}, // wrong IPv4 format
		{AFType1, "192.0.300.1:100", true},    // invalid IPv4 octet
		{AFType1, "[::1]:100", true},          // IPv6 format
		{AFType1, "1.2.3.4:001", true},        // leading zero in assigned number
		{AFType1, "1.2.3.4:65536", true},      // assigned number too large
		{AFType1, "1.2.3.4:-1", true},         // assigned number too small (<0)
		// Type 2 invalid
		{AFType2, "", true},               // empty
		{AFType2, ":", true},              // missing values
		{AFType2, ":100", true},           // missing ASN
		{AFType2, "100:", true},           // missing assigned number
		{AFType2, "001:100", true},        // leading zero in ASN
		{AFType2, "1:001", true},          // leading zero in assigned number
		{AFType2, "4294967295:100", true}, // ASN too large
		{AFType2, "4294967296:100", true}, // ASN too large
		{AFType2, "0:100", true},          // ASN too small (0 is not allowed for type 2)
		{AFType2, "65000:65536", true},    // assigned number too large
		{AFType2, "65000:abc", true},      // assigned number not numeric
		{AFType2, "abc:100", true},        // ASN not numeric
	}

	for _, tc := range tests {
		_, err := NewVPNIPv4Address(tc.afType, tc.value)
		if (err != nil) != tc.wantErr {
			t.Errorf("NewVPNIPv4Address(%q, %q) = %v, wantErr %v", tc.afType, tc.value, err, tc.wantErr)
		}
	}
}

func TestVPNIPv4Address_String(t *testing.T) {
	cases := []struct {
		afType  AFTYPE
		value   string
		wantStr string
	}{
		{AFType0, "65000:4294967294", "as2-nn4:65000:4294967294"},
		{AFType0, "65000:100", "as2-nn2:65000:100"},
		{AFType1, "192.168.1.1:65535", "ipv4-nn2:192.168.1.1:65535"},
		{AFType2, "4294967294:65535", "as4-nn2:4294967294:65535"},
	}

	for _, tc := range cases {
		addr, err := NewVPNIPv4Address(tc.afType, tc.value)
		if err != nil {
			t.Errorf("unexpected error for input (%q, %q): %v", tc.afType, tc.value, err)
			continue
		}
		got := addr.String()
		if got != tc.wantStr {
			t.Errorf("VPNIPv4Address.String() for (%q, %q) = %q, want %q", tc.afType, tc.value, got, tc.wantStr)
		}
	}
}
