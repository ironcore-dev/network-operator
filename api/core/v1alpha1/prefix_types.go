// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"encoding/json"
	"net/netip"

	"k8s.io/apimachinery/pkg/api/equality"
)

// IPPrefix represents an IP prefix in CIDR notation.
// It is used to define a range of IP addresses in a network.
//
// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Format=cidr
// +kubebuilder:validation:Example="192.168.1.0/24"
// +kubebuilder:validation:Example="2001:db8::/32"
// +kubebuilder:object:generate=false
type IPPrefix struct {
	netip.Prefix `json:"-"`
}

func ParsePrefix(s string) (IPPrefix, error) {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return IPPrefix{}, err
	}
	return IPPrefix{prefix}, nil
}

func MustParsePrefix(s string) IPPrefix {
	prefix := netip.MustParsePrefix(s)
	return IPPrefix{prefix}
}

// IsZero reports whether p represents the zero value
func (p IPPrefix) IsZero() bool {
	return !p.IsValid()
}

// Equal reports whether p and q are the same prefix.
// This method exists as a convenience for callers that need a direct comparison.
func (p IPPrefix) Equal(q IPPrefix) bool {
	return p.Prefix == q.Prefix
}

// MarshalJSON implements [json.Marshaler].
func (p IPPrefix) MarshalJSON() ([]byte, error) {
	if !p.IsValid() {
		return []byte("null"), nil
	}
	return json.Marshal(p.String())
}

// UnmarshalJSON implements [json.Unmarshaler].
func (p *IPPrefix) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" || str == "null" {
		*p = IPPrefix{}
		return nil
	}
	prefix, err := netip.ParsePrefix(str)
	if err != nil {
		return err
	}
	*p = IPPrefix{prefix}
	return nil
}

// IsPointToPoint reports whether the prefix indicates a point-to-point link.
// For IPv4, this means a /31 subnet mask as defined in [RFC 3021].
// For IPv6, this means a /127 subnet mask as defined in [RFC 6164].
//
// [RFC 3021]: https://datatracker.ietf.org/doc/html/rfc3021
// [RFC 6164]: https://datatracker.ietf.org/doc/html/rfc6164
func (p IPPrefix) IsPointToPoint() bool {
	if p.Addr().Is4() {
		return p.Bits() == 31
	}
	return p.Bits() == 127
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *IPPrefix) DeepCopyInto(out *IPPrefix) {
	*out = *in
}

// DeepCopy creates a deep copy of the IPPrefix
func (in *IPPrefix) DeepCopy() *IPPrefix {
	if in == nil {
		return nil
	}
	out := new(IPPrefix)
	in.DeepCopyInto(out)
	return out
}

func init() {
	// IPPrefix embeds [netip.Prefix] which contains unexported fields.
	// [equality.Semantic.DeepEqual] panics on unexported fields, so an
	// explicit equality function is registered in this package's init to
	// make any type containing IPPrefix safe to compare.
	if err := equality.Semantic.AddFunc(func(a, b IPPrefix) bool {
		return a.Equal(b)
	}); err != nil {
		panic(err)
	}
}
