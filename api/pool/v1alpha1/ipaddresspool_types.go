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

// IPAddressPoolSpec defines the desired state of IPAddressPool
type IPAddressPoolSpec struct {
	// Prefixes defines the CIDR ranges that can be allocated.
	// +required
	// +kubebuilder:validation:MinItems=1
	Prefixes []corev1alpha1.IPPrefix `json:"prefixes"`

	// ReclaimPolicy controls what happens to an allocation when a claim is deleted.
	// Recycle returns the allocation to the pool. Retain keeps it reserved.
	// Immutable.
	// +optional
	// +kubebuilder:default=Recycle
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="reclaimPolicy is immutable"
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// IPAddressPoolStatus defines the observed state of IPAddressPool.
type IPAddressPoolStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Allocated is the number of allocated IP addresses.
	// +optional
	Allocated string `json:"allocated,omitempty"`

	// Total is the number of allocatable IP addresses.
	// +optional
	Total string `json:"total,omitempty"`

	// conditions represent the current state of the IPAddressPool resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Allocations tracks which IP addresses are reserved by which claims.
	// +optional
	Allocations []IPAddressAllocation `json:"allocations,omitempty"`
}

// IPAddressAllocation represents a reserved IP address for a claim.
type IPAddressAllocation struct {
	// ClaimRef references the claim holding the allocation.
	// +required
	ClaimRef corev1alpha1.LocalObjectReference `json:"claimRef"`

	// ClaimUID is the UID of the claim holding the allocation.
	// +required
	ClaimUID types.UID `json:"claimUID"`

	// Address is the allocated IP address.
	// +required
	// +kubebuilder:validation:Format=ip
	Address string `json:"address"`

	// Retained indicates the allocation must not be reused after claim deletion.
	// +optional
	Retained bool `json:"retained,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ipaddresspools
// +kubebuilder:resource:singular=ipaddresspool
// +kubebuilder:resource:shortName=ippool
// +kubebuilder:printcolumn:name="Allocated",type=string,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// IPAddressPool is the Schema for the ipaddresspools API
type IPAddressPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IPAddressPoolSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IPAddressPoolStatus `json:"status,omitzero"`
}

// Total returns the total number of allocatable IP addresses in the pool.
func (p *IPAddressPool) Total() *big.Int {
	total := new(big.Int)
	for _, prefix := range p.Spec.Prefixes {
		bits := 32
		if prefix.Addr().Is6() {
			bits = 128
		}
		count := new(big.Int).Lsh(big.NewInt(1), uint(bits-prefix.Bits())) // #nosec G115
		total.Add(total, count)
	}
	return total
}

// Allocated returns the number of currently allocated IP addresses.
func (p *IPAddressPool) Allocated() int {
	return len(p.Status.Allocations)
}

// IsExhausted returns true if all available IP addresses have been allocated.
func (p *IPAddressPool) IsExhausted() bool {
	total := p.Total()
	if total.Sign() == 0 {
		return true
	}
	allocated := big.NewInt(int64(p.Allocated()))
	return allocated.Cmp(total) >= 0
}

// FindAllocation returns the ClaimAllocation for the given claim, or nil if not found.
func (p *IPAddressPool) FindAllocation(claim *Claim) *ClaimAllocation {
	for _, a := range p.Status.Allocations {
		if a.ClaimRef.Name == claim.Name && a.ClaimUID == claim.UID {
			return &ClaimAllocation{IPAddress: &a.Address, Value: a.Address}
		}
	}
	return nil
}

// GetConditions implements conditions.Getter.
func (p *IPAddressPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IPAddressPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// Allocate finds the first free IP address in the pool's prefixes and records it as an allocation for the given claim.
func (p *IPAddressPool) Allocate(claim *Claim) (*ClaimAllocation, error) {
	allocated := make(map[netip.Addr]struct{}, len(p.Status.Allocations))
	for _, a := range p.Status.Allocations {
		if addr, err := netip.ParseAddr(a.Address); err == nil {
			allocated[addr] = struct{}{}
		}
	}
	for _, prefix := range p.Spec.Prefixes {
		masked := prefix.Masked()
		for addr := masked.Addr(); masked.Contains(addr); addr = addr.Next() {
			if _, taken := allocated[addr]; !taken {
				addrStr := addr.String()
				p.Status.Allocations = append(p.Status.Allocations, IPAddressAllocation{
					ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
					ClaimUID: claim.UID,
					Address:  addrStr,
				})
				return &ClaimAllocation{
					IPAddress: &addrStr,
					Value:     addrStr,
				}, nil
			}
		}
	}
	return nil, ErrPoolExhausted
}

// AllocatePreferred reserves the specific IP address given by preferred for the claim.
// Returns ErrPreferredValueUnavailable if the value is outside the pool's configured
// prefixes or is already taken by another claim.
func (p *IPAddressPool) AllocatePreferred(claim *Claim, preferred string) (*ClaimAllocation, error) {
	addr, err := netip.ParseAddr(preferred)
	if err != nil {
		return nil, ErrPreferredValueUnavailable
	}
	inRange := false
	for _, prefix := range p.Spec.Prefixes {
		if prefix.Masked().Contains(addr) {
			inRange = true
			break
		}
	}
	if !inRange {
		return nil, ErrPreferredValueUnavailable
	}
	addrStr := addr.String()
	for _, a := range p.Status.Allocations {
		if a.Address == addrStr {
			return nil, ErrPreferredValueUnavailable
		}
	}
	p.Status.Allocations = append(p.Status.Allocations, IPAddressAllocation{
		ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
		ClaimUID: claim.UID,
		Address:  addrStr,
	})
	return &ClaimAllocation{IPAddress: &addrStr, Value: addrStr}, nil
}

// Reclaim applies the pool's reclaim policy for the given claim.
// On Recycle (default) the allocation is removed; on Retain it is kept with Retained=true.
func (p *IPAddressPool) Reclaim(claim *Claim) {
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

// IPAddressPoolList contains a list of IPAddressPool
type IPAddressPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPAddressPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IPAddressPool{}, &IPAddressPoolList{})
		return nil
	})
}
