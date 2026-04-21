// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// IndexRange represents an inclusive range of indices.
// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Pattern=`^[0-9]+\.\.[0-9]+$`
// +kubebuilder:object:generate=false
type IndexRange struct {
	Start int64 `json:"-"`
	End   int64 `json:"-"`
}

// ParseIndexRange parses a string in the format "start..end" into an [IndexRange].
func ParseIndexRange(s string) (IndexRange, error) {
	parts := strings.Split(s, "..")
	if len(parts) != 2 {
		return IndexRange{}, fmt.Errorf("invalid index range %q", s)
	}
	start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return IndexRange{}, fmt.Errorf("invalid index range start %q: %w", parts[0], err)
	}
	end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return IndexRange{}, fmt.Errorf("invalid index range end %q: %w", parts[1], err)
	}
	if start > end {
		return IndexRange{}, fmt.Errorf("invalid index range %q: start greater than end", s)
	}
	return IndexRange{Start: start, End: end}, nil
}

// MustParseIndexRange calls [ParseIndexRange] and panics if it returns an error.
func MustParseIndexRange(s string) IndexRange {
	r, err := ParseIndexRange(s)
	if err != nil {
		panic(err)
	}
	return r
}

// String implements [fmt.Stringer].
func (r IndexRange) String() string {
	return fmt.Sprintf("%d..%d", r.Start, r.End)
}

// MarshalJSON implements [json.Marshaler].
func (r IndexRange) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// UnmarshalJSON implements [json.Unmarshaler].
func (r *IndexRange) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" || str == "null" {
		*r = IndexRange{}
		return nil
	}
	parsed, err := ParseIndexRange(str)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

// DeepCopyInto copies all properties of this object into another object of the same type.
func (in *IndexRange) DeepCopyInto(out *IndexRange) {
	*out = *in
}

// DeepCopy creates a deep copy of the IndexRange.
func (in *IndexRange) DeepCopy() *IndexRange {
	if in == nil {
		return nil
	}
	out := new(IndexRange)
	in.DeepCopyInto(out)
	return out
}
