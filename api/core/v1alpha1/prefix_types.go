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

func (in *IPPrefix) First() netip.Addr {
	if !in.IsValid() {
		return netip.Addr{}
	}
	return in.Masked().Addr()
}

func (in *IPPrefix) Last() netip.Addr {
	if !in.IsValid() {
		return netip.Addr{}
	}
	net := in.Masked().Addr()
	if in.Bits() == 0 {
		if net.Is4() {
			return netip.AddrFrom4([4]byte{255, 255, 255, 255})
		}
		return netip.AddrFrom16([16]byte{
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
		})
	}
	if net.Is4() {
		a4 := net.As4()
		v := uint32(a4[0])<<24 | uint32(a4[1])<<16 | uint32(a4[2])<<8 | uint32(a4[3])
		hostBits := 32 - in.Bits()
		mask := uint32(1<<hostBits) - 1
		v |= mask
		return netip.AddrFrom4([4]byte{
			byte(v >> 24),
			byte(v >> 16),
			byte(v >> 8),
			byte(v),
		})
	}
	a16 := net.As16()
	hostBits := 128 - in.Bits()

	for i := range hostBits {
		byteIdx := 15 - i/8
		bit := byte(1 << (i % 8))
		a16[byteIdx] |= bit
	}
	return netip.AddrFrom16(a16)
}
