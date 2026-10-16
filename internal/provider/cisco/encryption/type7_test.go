// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"strings"
	"testing"
)

func TestEncryptType7(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		salt      int
		want      string
	}{
		//{name: "empty", plaintext: "", salt: 0, want: "00"},
		{name: "psk", plaintext: "HelloWorld", salt: 0,
			want: "002C160A08543C091D2D48"},
		{name: "psk", plaintext: "HelloWorld", salt: 50,
			want: "502F5D5B081C310B490702"},
		//{name: "psk with salt", plaintext: "GEEKSFORGEEKS", salt: 0, want: "GCYCZFMLYLEIM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := EncryptType7(tt.plaintext, tt.salt); err != nil || got != tt.want {
				t.Errorf("EncryptType7(%q, %d) = %q, want %q, err: %v", tt.plaintext, tt.salt, got, tt.want, err)
			}
		})
	}
}

func TestGenerateKey(t *testing.T) {
	// With salt 0 and a plaintext shorter than the key, GenerateKey repeats the
	// unrotated key and the trailing rotation is a no-op, so the result is simply
	// the first len(plaintext) characters of ciscoType7Key.
	shortPlaintext := "HelloWorld"
	wantShort := ciscoType7Key[:len(shortPlaintext)]

	// With a plaintext longer than the 53-byte key, the per-byte repetition wraps:
	// positions 0..52 are the full salted key (rotated by salt 5) and positions
	// 53..59 start over from its beginning.
	salt := 5
	overflowPlaintext := strings.Repeat("x", 60)
	wantOverflow := "kfoA,.iyewrkldJKDHSUBsgvca69834ncxv9873254k;fg87dsfd;kfoA,.i"

	tests := []struct {
		name      string
		salt      int
		plaintext string
		want      string
		wantErr   bool
	}{
		{name: "salt 0 matches start of key", salt: 0, plaintext: shortPlaintext, want: wantShort},
		{name: "overflow wraps key", salt: salt, plaintext: overflowPlaintext, want: wantOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateKey(tt.salt, tt.plaintext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateKey(%d, %q) error = %v, wantErr %v", tt.salt, tt.plaintext, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("GenerateKey(%d, %q) = %q, want %q", tt.salt, tt.plaintext, got, tt.want)
			}
		})
	}
}

func TestSaltCiscoKey(t *testing.T) {
	tests := []struct {
		name    string
		salt    int
		want    string
		wantErr bool
	}{
		{name: "salt 0 returns key unrotated", salt: 0, want: ciscoType7Key},
		{name: "salt 53 is out of range", salt: 53, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SaltCiscoKey(tt.salt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SaltCiscoKey(%d) error = %v, wantErr %v", tt.salt, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("SaltCiscoKey(%d) = %q, want %q", tt.salt, got, tt.want)
			}
		})
	}
}
