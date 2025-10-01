// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"encoding/json"
	"fmt"
)

// NOTE: Use json.Marshaler and json.Unmarshaler interfaces instead of the
// Marshaler interface for types that only need to customize their JSON
// representation and do not need to consider the capabilities of the target
// device. The Empty type is defined in RFC 7951 and has a specific JSON
// representation that is consistent across all devices and versions.
var (
	_ json.Marshaler   = (*Empty)(nil)
	_ json.Unmarshaler = (*Empty)(nil)
)

// Empty represents the built-in "empty" type as defined in RFC 7951.
// It differentiates between an existing empty value ([null]) and a
// non-existing value (null).
//
// RFC 7951: https://datatracker.ietf.org/doc/html/rfc7951#section-6.9
type Empty bool

// MarshalJSON implements json.Marshaler for Empty.
func (e Empty) MarshalJSON() ([]byte, error) {
	if !e {
		return []byte("null"), nil
	}
	return []byte("[null]"), nil
}

// UnmarshalJSON implements json.Unmarshaler for Empty.
func (e *Empty) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*e = false
		return nil
	}
	if string(b) != "[null]" {
		return fmt.Errorf("gnmiext: invalid empty value: %s", string(b))
	}
	*e = true
	return nil
}
