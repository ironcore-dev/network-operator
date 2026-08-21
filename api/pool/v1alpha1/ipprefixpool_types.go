// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"math/big"
	"net/netip"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IPPrefixPoolSpec defines the desired state of IPPrefixPool
type IPPrefixPoolSpec struct {
	// Prefixes defines the base prefixes to allocate from.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	Prefixes []corev1alpha1.IPPrefix `json:"prefixes"`

	// AllocationPrefixLength is the prefix length to allocate within each base prefix.
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=128
	AllocationPrefixLength int32 `json:"allocationPrefixLength"`

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
	Allocated int64 `json:"allocated"`

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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ipprefixpools
// +kubebuilder:resource:singular=ipprefixpool
// +kubebuilder:resource:shortName=pfxpool
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 43",message="pool name must not exceed 43 characters"

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
	target := int(p.Spec.AllocationPrefixLength)
	for _, prefix := range p.Spec.Prefixes {
		base := prefix.Masked()
		bits := 32
		if base.Addr().Is6() {
			bits = 128
		}
		if target < base.Bits() || target > bits {
			continue
		}
		count := new(big.Int).Lsh(big.NewInt(1), uint(target-base.Bits())) // #nosec G115
		total.Add(total, count)
	}
	return total
}

// IsExhausted returns true if all available prefixes have been allocated.
func (p *IPPrefixPool) IsExhausted() bool {
	total := p.Total()
	if total.Sign() == 0 {
		return true
	}
	allocated := big.NewInt(p.Status.Allocated)
	return allocated.Cmp(total) >= 0
}

// GetConditions implements conditions.Getter.
func (p *IPPrefixPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IPPrefixPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// ReclaimPolicy returns the pool's reclaim policy.
func (p *IPPrefixPool) ReclaimPolicy() ReclaimPolicy {
	return p.Spec.ReclaimPolicy
}

// ListAllocations lists all IPPrefix objects matching the given options.
func (p *IPPrefixPool) ListAllocations(ctx context.Context, c client.Client, opts ...client.ListOption) ([]Allocation, error) {
	list := &IPPrefixList{}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	allocs := make([]Allocation, len(list.Items))
	for i := range list.Items {
		allocs[i] = &list.Items[i]
	}
	return allocs, nil
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

// Allocate finds the first free sub-prefix and returns an IPPrefix allocation
// object for the given claim.
func (p *IPPrefixPool) Allocate(claim *Claim, existing []Allocation) (Allocation, error) {
	allocated := make(map[netip.Prefix]struct{}, len(existing))
	for _, obj := range existing {
		allocated[obj.(*IPPrefix).Spec.Prefix.Prefix] = struct{}{}
	}
	target := int(p.Spec.AllocationPrefixLength)
	for _, prefix := range p.Spec.Prefixes {
		masked := prefix.Masked()
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
				return &IPPrefix{
					TypeMeta: metav1.TypeMeta{
						APIVersion: GroupVersion.String(),
						Kind:       "IPPrefix",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", p.Name, sanitizeValue(candidate.String())),
						Namespace: p.Namespace,
					},
					Spec: IPPrefixSpec{
						PoolRef: corev1alpha1.TypedLocalObjectReference{
							APIVersion: GroupVersion.String(),
							Kind:       "IPPrefixPool",
							Name:       p.Name,
						},
						Prefix: corev1alpha1.IPPrefix{Prefix: candidate},
						ClaimRef: &ClaimRef{
							Name: claim.Name,
							UID:  claim.UID,
						},
					},
				}, nil
			}
		}
	}
	return nil, ErrPoolExhausted
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
