// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	Allocated int64 `json:"allocated"`

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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=indexpools
// +kubebuilder:resource:singular=indexpool
// +kubebuilder:resource:shortName=idxpool
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 43",message="pool name must not exceed 43 characters"

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
func (p *IndexPool) Total() int64 {
	var total int64
	for _, r := range p.Spec.Ranges {
		total += r.End - r.Start + 1
	}
	return total
}

// IsExhausted returns true if all available indices have been allocated.
func (p *IndexPool) IsExhausted() bool {
	return p.Status.Allocated >= p.Total()
}

// GetConditions implements conditions.Getter.
func (p *IndexPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IndexPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// ReclaimPolicy returns the pool's reclaim policy.
func (p *IndexPool) ReclaimPolicy() ReclaimPolicy {
	return p.Spec.ReclaimPolicy
}

// ListAllocations lists all Index objects matching the given options.
func (p *IndexPool) ListAllocations(ctx context.Context, c client.Client, opts ...client.ListOption) ([]Allocation, error) {
	list := &IndexList{}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	allocs := make([]Allocation, len(list.Items))
	for i := range list.Items {
		allocs[i] = &list.Items[i]
	}
	return allocs, nil
}

// Allocate finds the first free index across all ranges and returns an Index
// allocation object for the given claim.
func (p *IndexPool) Allocate(claim *Claim, existing []Allocation) (Allocation, error) {
	allocated := make(map[int64]struct{}, len(existing))
	for _, obj := range existing {
		idx := obj.(*Index)
		if idx.Spec.Index >= 0 {
			allocated[idx.Spec.Index] = struct{}{}
		}
	}
	for _, r := range p.Spec.Ranges {
		for idx := r.Start; idx <= r.End; idx++ {
			if _, taken := allocated[idx]; !taken {
				return &Index{
					TypeMeta: metav1.TypeMeta{
						APIVersion: GroupVersion.String(),
						Kind:       "Index",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%d", p.Name, idx),
						Namespace: p.Namespace,
					},
					Spec: IndexSpec{
						PoolRef: corev1alpha1.TypedLocalObjectReference{
							APIVersion: GroupVersion.String(),
							Kind:       "IndexPool",
							Name:       p.Name,
						},
						Index: idx,
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
