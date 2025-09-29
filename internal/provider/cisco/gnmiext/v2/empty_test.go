// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import "testing"

func TestEmpty_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		empty Empty
		want  string
	}{
		{
			name:  "valid empty (exists)",
			empty: Empty(true),
			want:  "[null]",
		},
		{
			name:  "invalid empty (does not exist)",
			empty: Empty(false),
			want:  "null",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.empty.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON() error = %v", err)
				return
			}
			if string(got) != test.want {
				t.Errorf("MarshalJSON() = %s, want %s", string(got), test.want)
			}
		})
	}
}

func TestEmpty_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Empty
		wantErr bool
	}{
		{
			name:    "valid [null] (exists)",
			input:   "[null]",
			want:    Empty(true),
			wantErr: false,
		},
		{
			name:    "valid null (does not exist)",
			input:   "null",
			want:    Empty(false),
			wantErr: false,
		},
		{
			name:    "empty input (does not exist)",
			input:   "",
			want:    Empty(false),
			wantErr: false,
		},
		{
			name:    "invalid json array",
			input:   "[1]",
			want:    Empty(false),
			wantErr: true,
		},
		{
			name:    "invalid json string",
			input:   `"test"`,
			want:    Empty(false),
			wantErr: true,
		},
		{
			name:    "invalid json object",
			input:   "{}",
			want:    Empty(false),
			wantErr: true,
		},
		{
			name:    "invalid json number",
			input:   "42",
			want:    Empty(false),
			wantErr: true,
		},
		{
			name:    "invalid array with multiple elements",
			input:   "[null, null]",
			want:    Empty(false),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var empty Empty
			err := empty.UnmarshalJSON([]byte(test.input))
			if (err != nil) != test.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !test.wantErr && empty != test.want {
				t.Errorf("UnmarshalJSON() got = %+v, want %+v", empty, test.want)
			}
		})
	}
}
