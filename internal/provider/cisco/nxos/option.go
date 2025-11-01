// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "encoding/json"

var (
	_ json.Marshaler   = Option[string]{}
	_ json.Unmarshaler = (*Option[string])(nil)
)

// Option is a generic optional value wrapper for NX-OS configurations.
// If unset, it will marshal to the string "DME_UNSET_PROPERTY_MARKER",
// effectively resetting the value in the configuration when sent to the device.
type Option[T comparable] struct {
	Value *T `json:"-"`
}

func NewOption[T comparable](v T) Option[T] {
	var zero T
	if v == zero {
		return Option[T]{}
	}
	return Option[T]{Value: &v}
}

func (o Option[T]) MarshalJSON() ([]byte, error) {
	if o.Value == nil {
		return []byte(`"DME_UNSET_PROPERTY_MARKER"`), nil
	}
	return json.Marshal(*o.Value)
}

func (o *Option[T]) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == `null` {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}
