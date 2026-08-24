// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"testing"
)

func TestIndexRangeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want IndexRange
	}{
		{
			name: "range string",
			data: `"10..20"`,
			want: IndexRange{Start: 10, End: 20},
		},
		{
			name: "single integer string",
			data: `"10"`,
			want: IndexRange{Start: 10, End: 10},
		},
		{
			name: "integer singleton",
			data: `10`,
			want: IndexRange{Start: 10, End: 10},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got IndexRange
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("json.Unmarshal() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestIndexRangeUnmarshalJSONRejectsInvalidStrings(t *testing.T) {
	for _, data := range []string{`""`, `"null"`} {
		t.Run(data, func(t *testing.T) {
			var got IndexRange
			if err := json.Unmarshal([]byte(data), &got); err == nil {
				t.Fatalf("json.Unmarshal(%s) succeeded, want error", data)
			}
		})
	}
}

func TestIndexRangeMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   IndexRange
		want string
	}{
		{
			name: "single integer",
			in:   IndexRange{Start: 10, End: 10},
			want: `10`,
		},
		{
			name: "range string",
			in:   IndexRange{Start: 10, End: 20},
			want: `"10..20"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("json.Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}
