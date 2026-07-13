// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/pmezard/go-difflib/difflib"
)

// CompareJSON compares two JSON strings and returns an error if they are not equal.
// For comparison, it unmarshals both into interface{} and uses reflect.DeepEqual
// after sorting any arrays and removing empty arrays/objects to ignore ordering
// and cleanup artifacts.
func CompareJSON(got, want string) error {
	var gotObj, wantObj any
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		return fmt.Errorf("failed to unmarshal got JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		return fmt.Errorf("failed to unmarshal want JSON: %w", err)
	}

	// Normalize both objects (sort arrays, remove empty containers)
	gotObj = normalizeJSON(gotObj)
	wantObj = normalizeJSON(wantObj)

	if !reflect.DeepEqual(gotObj, wantObj) {
		// Pretty-print both for readable diff
		gotPretty, _ := json.MarshalIndent(gotObj, "", "  ")
		wantPretty, _ := json.MarshalIndent(wantObj, "", "  ")

		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(wantPretty)),
			B:        difflib.SplitLines(string(gotPretty)),
			FromFile: "want",
			ToFile:   "got",
			Context:  3,
		}
		diffStr, _ := difflib.GetUnifiedDiffString(diff)
		return fmt.Errorf("JSON mismatch:\n%s", diffStr)
	}
	return nil
}

// normalizeJSON recursively sorts arrays and removes empty arrays/objects
// to make comparison order-independent and ignore cleanup artifacts.
func normalizeJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			normalized := normalizeJSON(v)
			// Skip empty maps and empty arrays
			if !isEmpty(normalized) {
				result[k] = normalized
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []any:
		var result []any
		for _, elem := range val {
			normalized := normalizeJSON(elem)
			if !isEmpty(normalized) {
				result = append(result, normalized)
			}
		}
		if len(result) == 0 {
			return nil
		}
		// Sort the array by JSON representation
		sort.Slice(result, func(i, j int) bool {
			bi, _ := json.Marshal(result[i]) //nolint:errcheck // sorting comparison, errors treated as equal
			bj, _ := json.Marshal(result[j]) //nolint:errcheck // sorting comparison, errors treated as equal
			return string(bi) < string(bj)
		})
		return result
	default:
		return v
	}
}

// isEmpty checks if a value is an empty map, empty array, or nil.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	}
	return false
}
