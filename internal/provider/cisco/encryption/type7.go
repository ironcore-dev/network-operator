// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"fmt"
	"strings"
)

// ciscoType7Key is Cisco's fixed Vigenère translation table (the key) used by
// service-password-encryption (Type 7). All 53 bytes are used.
const ciscoType7Key = "dsfd;kfoA,.iyewrkldJKDHSUBsgvca69834ncxv9873254k;fg87"

// EncryptType7 encodes plaintext as a Cisco Type 7 password string.
//
// Cisco Type 7 is a weak, reversible obfuscation (NOT real encryption) used by
// service-password-encryption. ciscoType7Key is the fixed Vigenère key: each
// plaintext byte is XORed against a byte of the key, and the result is emitted
// as a two-digit decimal seed index (the salt) followed by the uppercase hex of
// each XORed byte.
// Devices such as IOS-XR require it because leaves like the
// key-chain password (xr:Proprietary-password) reject plaintext.
func EncryptType7(plaintext string, salt int) (string, error) {
	var b strings.Builder

	key, err := GenerateKey(salt, plaintext)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "%02d", salt)
	for i := range plaintext {
		fmt.Fprintf(&b, "%02X", plaintext[i]^key[i])
	}
	return b.String(), nil
}

// SaltCiscoKey returns the Cisco Type 7 key rotated by the given salt.
// For deterministic output, pass a fixed salt
func SaltCiscoKey(salt int) (string, error) {
	if salt < 0 || salt >= len(ciscoType7Key) {
		return "", fmt.Errorf("invalid salt: %d", salt)
	}
	return ciscoType7Key[salt:] + ciscoType7Key[:salt], nil
}

// GenerateKey produces a key of the same length as the plaintext by repeating the salted Cisco key.
func GenerateKey(salt int, plaintext string) (string, error) {
	saltedKey, err := SaltCiscoKey(salt)
	if err != nil {
		return "", err
	}
	key := ""
	for i := range plaintext {
		key += string(saltedKey[i%len(saltedKey)])
	}
	return key, nil
}
