// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"encoding/json"
	"reflect"
	"strings"
)

// sensitiveTag marks a struct field whose value must be masked before logging.
// Fields tagged `sensitive:"true"` still carry their real value in the gNMI Set
// payload sent to the device; only the logged representation is redacted.
const sensitiveTag = "sensitive"

const redactedValue = "****"

// redactPayload returns a loggable JSON representation of a gNMI payload value
// with every field tagged `sensitive:"true"` replaced by redactedValue. The
// original value is never mutated. On failure it falls back to a fixed
// placeholder rather than risking leaking the raw value.
func redactPayload(el any) string {
	b, err := json.Marshal(redactValue(reflect.ValueOf(el)))
	if err != nil {
		return "<redacted>"
	}
	return string(b)
}

// redactValue returns a JSON-marshalable copy of rv with sensitive struct
// fields masked. Non-struct values are returned as-is via their interface.
func redactValue(rv reflect.Value) any {
	if !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return redactValue(rv.Elem())
	case reflect.Struct:
		out := make(map[string]any, rv.NumField())
		rt := rv.Type()
		for i := range rt.NumField() {
			field := rt.Field(i)
			if !field.IsExported() {
				continue
			}
			name, omit := jsonFieldName(field)
			if name == "-" {
				continue
			}
			fv := rv.Field(i)
			if omit && fv.IsZero() {
				continue
			}
			if field.Tag.Get(sensitiveTag) == "true" {
				out[name] = redactedValue
				continue
			}
			out[name] = redactValue(fv)
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			out[i] = redactValue(rv.Index(i))
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, k := range rv.MapKeys() {
			out[k.String()] = redactValue(rv.MapIndex(k))
		}
		return out
	default:
		return rv.Interface()
	}
}

// jsonFieldName returns the JSON object key for a struct field and whether the
// field is omitted when empty (json:",omitempty"/",omitzero"). A field with no
// json tag falls back to its Go name, matching encoding/json.
func jsonFieldName(field reflect.StructField) (name string, omitEmpty bool) {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return field.Name, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" || opt == "omitzero" {
			omitEmpty = true
		}
	}
	return name, omitEmpty
}
