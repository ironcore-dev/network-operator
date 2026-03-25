// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IndexPoolSpec defines the desired state of IndexPool
type IndexPoolSpec struct {
	// Ranges defines the inclusive index ranges that can be allocated.
	// Example: "64512..65534".
	// +required
	// +kubebuilder:validation:MinItems=1
	Ranges []corev1alpha1.IndexRange `json:"ranges"`

	// ReclaimPolicy controls what happens to an allocation when a claim is deleted.
	// Recycle returns the allocation to the pool. Retain keeps it reserved.
	// Immutable.
	// +optional
	// +kubebuilder:default=Recycle
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="reclaimPolicy is immutable"
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// IndexPoolStatus defines the observed state of IndexPool.
type IndexPoolStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Allocated is the number of allocated indices.
	// +optional
	Allocated string `json:"allocated,omitempty"`

	// Total is the number of allocatable indices.
	// +optional
	Total string `json:"total,omitempty"`

	// conditions represent the current state of the IndexPool resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Allocations tracks which indices are reserved by which claims.
	// +optional
	Allocations []IndexAllocation `json:"allocations,omitempty"`
}

// IndexAllocation represents a reserved index for a claim.
type IndexAllocation struct {
	// ClaimRef references the claim holding the allocation.
	// +required
	ClaimRef corev1alpha1.LocalObjectReference `json:"claimRef"`

	// ClaimUID is the UID of the claim holding the allocation.
	// +required
	ClaimUID types.UID `json:"claimUID"`

	// Index is the allocated value.
	// +required
	Index uint64 `json:"index"`

	// Retained indicates the allocation must not be reused after claim deletion.
	// +optional
	Retained bool `json:"retained,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=indexpools
// +kubebuilder:resource:singular=indexpool
// +kubebuilder:resource:shortName=idxpool
// +kubebuilder:printcolumn:name="Allocated",type=string,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// IndexPool is the Schema for the indexpools API
type IndexPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IndexPoolSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IndexPoolStatus `json:"status,omitzero"`
}

// Total returns the total number of allocatable indices in the pool.
func (p *IndexPool) Total() uint64 {
	var total uint64
	for _, r := range p.Spec.Ranges {
		total += r.End - r.Start + 1
	}
	return total
}

// Allocated returns the number of currently allocated indices.
func (p *IndexPool) Allocated() int {
	return len(p.Status.Allocations)
}

// IsExhausted returns true if all available indices have been allocated.
func (p *IndexPool) IsExhausted() bool {
	return uint64(p.Allocated()) >= p.Total() // #nosec G115
}

// FindAllocation returns the ClaimAllocation for the given claim, or nil if not found.
func (p *IndexPool) FindAllocation(claim *Claim) *ClaimAllocation {
	for _, a := range p.Status.Allocations {
		if a.ClaimRef.Name == claim.Name && a.ClaimUID == claim.UID {
			return &ClaimAllocation{Index: &a.Index, Value: strconv.FormatUint(a.Index, 10)}
		}
	}
	return nil
}

// GetConditions implements conditions.Getter.
func (p *IndexPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IndexPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// Allocate finds the first free index across all ranges, records the allocation,
// and returns a ClaimAllocation describing the reserved index.
func (p *IndexPool) Allocate(claim *Claim) (*ClaimAllocation, error) {
	allocated := make(map[uint64]struct{}, len(p.Status.Allocations))
	for _, a := range p.Status.Allocations {
		allocated[a.Index] = struct{}{}
	}
	for _, r := range p.Spec.Ranges {
		for idx := r.Start; idx <= r.End; idx++ {
			if _, taken := allocated[idx]; !taken {
				p.Status.Allocations = append(p.Status.Allocations, IndexAllocation{
					ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
					ClaimUID: claim.UID,
					Index:    idx,
				})
				return &ClaimAllocation{
					Index: &idx,
					Value: strconv.FormatUint(idx, 10),
				}, nil
			}
		}
	}
	return nil, ErrPoolExhausted
}

// AllocatePreferred reserves the specific index given by preferred for the claim.
// Returns ErrPreferredValueUnavailable if the value is outside the pool's configured
// ranges or is already taken by another claim.
func (p *IndexPool) AllocatePreferred(claim *Claim, preferred string) (*ClaimAllocation, error) {
	idx, err := strconv.ParseUint(preferred, 10, 64)
	if err != nil {
		return nil, ErrPreferredValueUnavailable
	}
	inRange := false
	for _, r := range p.Spec.Ranges {
		if idx >= r.Start && idx <= r.End {
			inRange = true
			break
		}
	}
	if !inRange {
		return nil, ErrPreferredValueUnavailable
	}
	for _, a := range p.Status.Allocations {
		if a.Index == idx {
			return nil, ErrPreferredValueUnavailable
		}
	}
	p.Status.Allocations = append(p.Status.Allocations, IndexAllocation{
		ClaimRef: corev1alpha1.LocalObjectReference{Name: claim.Name},
		ClaimUID: claim.UID,
		Index:    idx,
	})
	return &ClaimAllocation{Index: &idx, Value: strconv.FormatUint(idx, 10)}, nil
}

// Reclaim applies the pool's reclaim policy for the given claim.
// On Recycle (default) the allocation is removed; on Retain it is kept with Retained=true.
func (p *IndexPool) Reclaim(claim *Claim) {
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

// IndexPoolList contains a list of IndexPool
type IndexPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IndexPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IndexPool{}, &IndexPoolList{})
		return nil
	})
}
