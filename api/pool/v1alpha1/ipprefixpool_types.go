// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math/big"
	"net/netip"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IPPrefixPoolSpec defines the desired state of IPPrefixPool
type IPPrefixPoolSpec struct {
	// Prefixes defines the base prefixes and target prefix lengths to allocate from.
	// +required
	// +kubebuilder:validation:MinItems=1
	Prefixes []IPPrefixPoolPrefix `json:"prefixes"`

	// ReclaimPolicy controls what happens to an allocation when a claim is deleted.
	// Recycle returns the allocation to the pool. Retain keeps it reserved.
	// Immutable.
	// +optional
	// +kubebuilder:default=Recycle
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="reclaimPolicy is immutable"
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// IPPrefixPoolStatus defines the observed state of IPPrefixPool.
type IPPrefixPoolStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Allocated is the number of allocated prefixes.
	// +optional
	Allocated string `json:"allocated,omitempty"`

	// Total is the number of allocatable prefixes.
	// +optional
	Total string `json:"total,omitempty"`

	// conditions represent the current state of the IPPrefixPool resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Allocations tracks which prefixes are reserved by which claims.
	// +optional
	Allocations []IPPrefixAllocation `json:"allocations,omitempty"`
}

// IPPrefixPoolPrefix defines a pool prefix and the target length to allocate.
type IPPrefixPoolPrefix struct {
	// Prefix is the base prefix to allocate prefixes from.
	// +required
	Prefix corev1alpha1.IPPrefix `json:"prefix"`

	// PrefixLength is the prefix length to allocate within the base prefix.
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=128
	PrefixLength int32 `json:"prefixLength"`
}

// IPPrefixAllocation represents a reserved prefix for a claim.
type IPPrefixAllocation struct {
	// ClaimRef references the claim holding the allocation.
	// +required
	ClaimRef corev1alpha1.LocalObjectReference `json:"claimRef"`

	// ClaimUID is the UID of the claim holding the allocation.
	// +required
	ClaimUID types.UID `json:"claimUID"`

	// Prefix is the allocated prefix.
	// +required
	Prefix corev1alpha1.IPPrefix `json:"prefix"`

	// Retained indicates the allocation must not be reused after claim deletion.
	// +optional
	Retained bool `json:"retained,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ipprefixpools
// +kubebuilder:resource:singular=ipprefixpool
// +kubebuilder:resource:shortName=pfxpool
// +kubebuilder:printcolumn:name="Allocated",type=string,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// IPPrefixPool is the Schema for the ipprefixpools API
type IPPrefixPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IPPrefixPoolSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IPPrefixPoolStatus `json:"status,omitzero"`
}

// Total returns the total number of allocatable prefixes in the pool.
func (p *IPPrefixPool) Total() *big.Int {
	total := new(big.Int)
	for _, poolPrefix := range p.Spec.Prefixes {
		base := poolPrefix.Prefix.Masked()
		bits := 32
		if base.Addr().Is6() {
			bits = 128
		}
		target := int(poolPrefix.PrefixLength)
		if target < base.Bits() || target > bits {
			continue
		}
		count := new(big.Int).Lsh(big.NewInt(1), uint(target-base.Bits())) // #nosec G115
		total.Add(total, count)
	}
	return total
}

// Allocated returns the number of currently allocated prefixes.
func (p *IPPrefixPool) Allocated() int {
	return len(p.Status.Allocations)
}

// IsExhausted returns true if all available prefixes have been allocated.
func (p *IPPrefixPool) IsExhausted() bool {
	total := p.Total()
	if total.Sign() == 0 {
		return true
	}
	allocated := big.NewInt(int64(p.Allocated()))
	return allocated.Cmp(total) >= 0
}

// FindAllocation returns the ClaimAllocation for the given claim, or nil if not found.
func (p *IPPrefixPool) FindAllocation(claim *Claim) *ClaimAllocation {
	for _, a := range p.Status.Allocations {
		if a.ClaimRef.Name == claim.Name && a.ClaimUID == claim.UID {
			return &ClaimAllocation{Prefix: &a.Prefix, Value: a.Prefix.String()}
		}
	}
	return nil
}

// GetConditions implements conditions.Getter.
func (p *IPPrefixPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IPPrefixPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// stepAddr advances addr by 2^n by treating the address as a big-endian
// 128-bit integer stored in a [16]byte array.
func stepAddr(addr netip.Addr, n int) netip.Addr {
	b := addr.As16()
	// Add 2^n to the 128-bit value. Work from the least significant byte
	// (index 15) upward, carrying as needed.
	//
	// Example: advancing 10.0.0.192 by 2^6 (n=6, one /26 step):
	//	carry = 1 << (6%8) = 64,  start at byte 15 - 6/8 = 15
	//	byte 15: 192 + 64 = 256 → store 0, carry 1
	//	byte 14:   0 +  1 =   1 → store 1, carry 0  (stop)
	//	result: 10.0.1.0
	carry := uint16(1) << (n % 8)
	for i := 15 - n/8; i >= 0 && carry > 0; i-- {
		sum := uint16(b[i]) + carry
		b[i] = uint8(sum) // #nosec G115
		carry = sum >> 8
	}
	if carry > 0 {
		return netip.Addr{}
	}
	result := netip.AddrFrom16(b)
	if addr.Is4() {
		result = result.Unmap()
		if !result.Is4() {
			return netip.Addr{}
		}
	}
	return result
}

// Allocate finds the first free sub-prefix and records it in the pool's status.
func (p *IPPrefixPool) Allocate(claim *Claim) (*ClaimAllocation, error) {
	allocated := make(map[netip.Prefix]struct{}, len(p.Status.Allocations))
	for _, a := range p.Status.Allocations {
		allocated[a.Prefix.Prefix] = struct{}{}
	}
	for _, prefix := range p.Spec.Prefixes {
		masked := prefix.Prefix.Masked()
		target := int(prefix.PrefixLength)
		bits := 32
		if masked.Addr().Is6() {
			bits = 128
		}
		if target < masked.Bits() || target > bits {
			continue
		}
		stepBits := bits - target
		for addr := masked.Addr(); masked.Contains(addr); addr = stepAddr(addr, stepBits) {
			candidate := netip.PrefixFrom(addr, target)
			if _, taken := allocated[candidate]; !taken {
				prefix := corev1alpha1.IPPrefix{Prefix: candidate}
				p.Status.Allocations = append(p.Status.Allocations, IPPrefixAllocation{
					ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
					ClaimUID: claim.UID,
					Prefix:   prefix,
				})
				return &ClaimAllocation{
					Prefix: &prefix,
					Value:  prefix.String(),
				}, nil
			}
		}
	}
	return nil, ErrPoolExhausted
}

// AllocatePreferred reserves the specific prefix given by preferred for the claim.
// Returns ErrPreferredValueUnavailable if the value is outside the pool's configured
// prefixes or is already taken by another claim.
func (p *IPPrefixPool) AllocatePreferred(claim *Claim, preferred string) (*ClaimAllocation, error) {
	candidate, err := netip.ParsePrefix(preferred)
	if err != nil {
		return nil, ErrPreferredValueUnavailable
	}
	candidate = candidate.Masked()
	inRange := false
	for _, pp := range p.Spec.Prefixes {
		if int32(candidate.Bits()) == pp.PrefixLength && pp.Prefix.Masked().Contains(candidate.Addr()) { // #nosec G115
			inRange = true
			break
		}
	}
	if !inRange {
		return nil, ErrPreferredValueUnavailable
	}
	for _, a := range p.Status.Allocations {
		if a.Prefix.Prefix == candidate {
			return nil, ErrPreferredValueUnavailable
		}
	}
	prefix := corev1alpha1.IPPrefix{Prefix: candidate}
	p.Status.Allocations = append(p.Status.Allocations, IPPrefixAllocation{
		ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
		ClaimUID: claim.UID,
		Prefix:   prefix,
	})
	return &ClaimAllocation{Prefix: &prefix, Value: prefix.String()}, nil
}

// Reclaim applies the pool's reclaim policy for the given claim.
// On Recycle (default) the allocation is removed; on Retain it is kept with Retained=true.
func (p *IPPrefixPool) Reclaim(claim *Claim) {
	for i := range p.Status.Allocations {
		a := &p.Status.Allocations[i]
		if a.ClaimRef.Name != claim.Name || a.ClaimUID != claim.UID {
			continue
		}
		if p.Spec.ReclaimPolicy == ReclaimPolicyRetain {
			a.Retained = true
			continue
		}
		p.Status.Allocations = append(p.Status.Allocations[:i], p.Status.Allocations[i+1:]...)
		return
	}
}

// +kubebuilder:object:root=true

// IPPrefixPoolList contains a list of IPPrefixPool
type IPPrefixPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPPrefixPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IPPrefixPool{}, &IPPrefixPoolList{})
		return nil
	})
}
