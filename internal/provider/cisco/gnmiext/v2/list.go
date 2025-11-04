// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// Keyed represents a configuration item that can provide its key value.
// This is typically implemented by YANG list items to extract their key
// for use in the List type.
type Keyed[K comparable] interface {
	// Key returns the key value for this list item.
	Key() K
}

// List represents a YANG list node as a map in Go, ensuring order-independent
// comparison while marshaling to/from JSON arrays.
//
// YANG list nodes are uniquely identified by their key leafs, and the ordering
// of entries does not matter. By using a map internally, reflect.DeepEqual will
// correctly compare two List instances regardless of insertion order.
//
// The type parameters are:
//   - K: the key type (must be comparable)
//   - V: the value type (must implement both Configurable and Keyed[K])
//
// Example usage:
//
//	type DomainItem struct {
//	    Name string
//	    Value int
//	}
//
//	func (d *DomainItem) Key() string { return d.Name }
//	func (d *DomainItem) XPath() string { return fmt.Sprintf("dom[name=%s]", d.Name) }
//
//	type Config struct {
//	    Domains List[string, *DomainItem]
//	}
type List[K comparable, V interface {
	Configurable
	Keyed[K]
}] map[K]V

// MarshalJSON implements the json.Marshaler interface.
// It converts the map to a slice of values and marshals it as a JSON array.
// The order of elements in the resulting array is not guaranteed, but this
// is acceptable for YANG list nodes where order does not matter.
//
// Maintains the distinction between nil (marshals to "null") and an empty
// initialized list (marshals to "[]").
func (l List[K, V]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}

	if len(l) == 0 {
		return []byte("[]"), nil
	}

	return json.Marshal(slices.Collect(maps.Values(l)))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It unmarshals a JSON array into the map, using each item's Key() method
// to determine the map key.
func (l *List[K, V]) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		*l = nil
		return nil
	}

	// Unmarshal into a slice first
	var slice []V
	if err := json.Unmarshal(data, &slice); err != nil {
		return fmt.Errorf("failed to unmarshal list: %w", err)
	}

	// Initialize the map if needed
	if *l == nil {
		*l = make(List[K, V], len(slice))
	}

	// Convert slice to map using the Key() method
	for _, item := range slice {
		key := item.Key()
		(*l)[key] = item
	}

	return nil
}

// Len returns the number of items in the list.
func (l List[K, V]) Len() int {
	return len(l)
}

// Get retrieves an item from the list by its key.
// Returns the item and true if found, or the zero value and false if not found.
func (l List[K, V]) Get(key K) (V, bool) {
	v, ok := l[key]
	return v, ok
}

// Set adds or updates an item in the list.
// The key is extracted from the item using its Key() method.
func (l List[K, V]) Set(item V) {
	key := item.Key()
	l[key] = item
}

// Delete removes an item from the list by its key.
func (l List[K, V]) Delete(key K) {
	delete(l, key)
}
