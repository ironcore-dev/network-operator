// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"encoding/json"
	"net/netip"

	"k8s.io/apimachinery/pkg/api/equality"
)

// IPAddr represents a single IP address (IPv4 or IPv6).
//
// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Format=ip
// +kubebuilder:validation:Example="192.168.1.1"
// +kubebuilder:validation:Example="2001:db8::1"
// +kubebuilder:object:generate=false
type IPAddr struct {
	netip.Addr `json:"-"`
}

func ParseAddr(s string) (IPAddr, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return IPAddr{}, err
	}
	return IPAddr{addr}, nil
}

func MustParseAddr(s string) IPAddr {
	return IPAddr{netip.MustParseAddr(s)}
}

// IsZero reports whether a represents the zero value.
func (a IPAddr) IsZero() bool {
	return !a.IsValid()
}

// Equal reports whether a and b represent the same address.
func (a IPAddr) Equal(b IPAddr) bool {
	return a.Addr == b.Addr
}

// MarshalJSON implements [json.Marshaler].
func (a IPAddr) MarshalJSON() ([]byte, error) {
	if !a.IsValid() {
		return []byte("null"), nil
	}
	return json.Marshal(a.String())
}

// UnmarshalJSON implements [json.Unmarshaler].
func (a *IPAddr) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" || str == "null" {
		*a = IPAddr{}
		return nil
	}
	addr, err := netip.ParseAddr(str)
	if err != nil {
		return err
	}
	*a = IPAddr{addr}
	return nil
}

// DeepCopyInto copies all properties of this object into another object of the same type.
func (in *IPAddr) DeepCopyInto(out *IPAddr) {
	*out = *in
}

// DeepCopy creates a deep copy of the IPAddr.
func (in *IPAddr) DeepCopy() *IPAddr {
	if in == nil {
		return nil
	}
	out := new(IPAddr)
	in.DeepCopyInto(out)
	return out
}

func init() {
	// IPAddr embeds [netip.Addr] which contains unexported fields.
	// [equality.Semantic.DeepEqual] panics on unexported fields, so an
	// explicit equality function is registered here.
	if err := equality.Semantic.AddFunc(func(a, b IPAddr) bool {
		return a.Equal(b)
	}); err != nil {
		panic(err)
	}
}
