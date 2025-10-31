// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"encoding/json"
	"net/netip"
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
