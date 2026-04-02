// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"
)

// Test item that implements Configurable and Keyed
type testItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func (t *testItem) Key() string {
	return t.Name
}

func (t *testItem) XPath() string {
	return fmt.Sprintf("test-items/item[name=%s]", t.Name)
}

// Test container with a List field
type testContainer struct {
	Items List[string, *testItem] `json:"items"`
}

func TestList_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		list    List[string, *testItem]
		wantErr bool
		check   func(t *testing.T, data []byte)
	}{
		{
			name: "empty list",
			list: List[string, *testItem]{},
			check: func(t *testing.T, data []byte) {
				if string(data) != "[]" {
					t.Errorf("expected [], got %s", string(data))
				}
			},
		},
		{
			name: "nil list",
			list: nil,
			check: func(t *testing.T, data []byte) {
				if string(data) != "null" {
					t.Errorf("expected null, got %s", string(data))
				}
			},
		},
		{
			name: "single item",
			list: List[string, *testItem]{
				"item1": {Name: "item1", Value: 100},
			},
			check: func(t *testing.T, data []byte) {
				var items []testItem
				if err := json.Unmarshal(data, &items); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if len(items) != 1 {
					t.Fatalf("expected 1 item, got %d", len(items))
				}
				if items[0].Name != "item1" || items[0].Value != 100 {
					t.Errorf("unexpected item: %+v", items[0])
				}
			},
		},
		{
			name: "multiple items",
			list: List[string, *testItem]{
				"item1": {Name: "item1", Value: 100},
				"item2": {Name: "item2", Value: 200},
				"item3": {Name: "item3", Value: 300},
			},
			check: func(t *testing.T, data []byte) {
				var items []testItem
				if err := json.Unmarshal(data, &items); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if len(items) != 3 {
					t.Fatalf("expected 3 items, got %d", len(items))
				}
				// Build a map to check all items are present (order doesn't matter)
				found := make(map[string]int)
				for _, item := range items {
					found[item.Name] = item.Value
				}
				if found["item1"] != 100 || found["item2"] != 200 || found["item3"] != 300 {
					t.Errorf("unexpected items: %+v", found)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.list.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, data)
			}
		})
	}
}

func TestList_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    List[string, *testItem]
		wantErr bool
	}{
		{
			name: "empty array",
			data: "[]",
			want: List[string, *testItem]{},
		},
		{
			name: "null",
			data: "null",
			want: nil,
		},
		{
			name: "single item",
			data: `[{"name":"item1","value":100}]`,
			want: List[string, *testItem]{
				"item1": {Name: "item1", Value: 100},
			},
		},
		{
			name: "multiple items",
			data: `[{"name":"item1","value":100},{"name":"item2","value":200},{"name":"item3","value":300}]`,
			want: List[string, *testItem]{
				"item1": {Name: "item1", Value: 100},
				"item2": {Name: "item2", Value: 200},
				"item3": {Name: "item3", Value: 300},
			},
		},
		{
			name: "items in different order",
			data: `[{"name":"item3","value":300},{"name":"item1","value":100},{"name":"item2","value":200}]`,
			want: List[string, *testItem]{
				"item1": {Name: "item1", Value: 100},
				"item2": {Name: "item2", Value: 200},
				"item3": {Name: "item3", Value: 300},
			},
		},
		{
			name:    "invalid JSON",
			data:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got List[string, *testItem]
			err := got.UnmarshalJSON([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalJSON() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestList_Methods(t *testing.T) {
	list := List[string, *testItem]{
		"item1": {Name: "item1", Value: 100},
		"item2": {Name: "item2", Value: 200},
	}

	// Test Len
	if list.Len() != 2 {
		t.Errorf("Len() = %d, want 2", list.Len())
	}

	// Test Get
	item, ok := list.Get("item1")
	if !ok || item.Value != 100 {
		t.Error("Get() failed to retrieve item1")
	}

	_, ok = list.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for nonexistent key")
	}

	// Test Set
	list.Set(&testItem{Name: "item3", Value: 300})
	if list.Len() != 3 {
		t.Errorf("After Set(), Len() = %d, want 3", list.Len())
	}

	// Test Delete
	list.Delete("item2")
	if list.Len() != 2 {
		t.Errorf("After Delete(), Len() = %d, want 2", list.Len())
	}

	// Test maps.Keys from stdlib
	keys := slices.Collect(maps.Keys(list))
	if len(keys) != 2 {
		t.Errorf("maps.Keys() returned %d keys, want 2", len(keys))
	}

	// Test maps.Values from stdlib
	values := slices.Collect(maps.Values(list))
	if len(values) != 2 {
		t.Errorf("maps.Values() returned %d values, want 2", len(values))
	}
}

func TestList_InContainer(t *testing.T) {
	container := testContainer{
		Items: List[string, *testItem]{
			"item1": {Name: "item1", Value: 100},
			"item2": {Name: "item2", Value: 200},
		},
	}

	// Marshal
	data, err := json.Marshal(container)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal
	var result testContainer
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Compare
	if !reflect.DeepEqual(container, result) {
		t.Errorf("container round trip failed")
	}
}
