// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// Allocation is an interface implemented by allocation objects (Index, IPAddress, IPPrefix).
// +kubebuilder:object:generate=false
type Allocation interface {
	client.Object

	// Value returns the allocated value as a string.
	Value() string

	// ClaimRef returns the ClaimRef bound to this allocation, or nil if unbound.
	ClaimRef() *ClaimRef

	// SetClaimRef sets or clears the ClaimRef on this allocation.
	SetClaimRef(*ClaimRef)
}

// Pool is an interface that abstracts over the different pool types (IndexPool, IPAddressPool, IPPrefixPool) that a Claim can reference.
// +kubebuilder:object:generate=false
type Pool interface {
	client.Object

	// IsExhausted reports whether all allocatable resources in the pool are taken.
	IsExhausted() bool

	// ReclaimPolicy returns the pool's configured reclaim policy.
	ReclaimPolicy() ReclaimPolicy

	// ListAllocations lists allocation objects matching the given options.
	ListAllocations(ctx context.Context, c client.Client, opts ...client.ListOption) ([]Allocation, error)

	// Allocate picks the next free value from the pool's ranges/prefixes,
	// given the existing allocations for this pool, and returns a new Allocation
	// with a ready-to-create object, deterministic name, and the allocated value.
	// Returns ErrPoolExhausted when no free value remains.
	Allocate(claim *Claim, existing []Allocation) (Allocation, error)
}

// ClaimSpec defines the desired state of Claim.
type ClaimSpec struct {
	// PoolRef references the allocation pool to allocate from.
	// PoolRef is immutable once set.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="poolRef is immutable"
	PoolRef corev1alpha1.TypedLocalObjectReference `json:"poolRef"`
}

// ClaimStatus defines the observed state of Claim.
type ClaimStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Claim resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AllocationRef references the bound allocation object (Index, IPAddress, or IPPrefix).
	// Set by the claim controller after successful binding.
	// +optional
	AllocationRef *corev1alpha1.TypedLocalObjectReference `json:"allocationRef,omitempty"`

	// Value is the allocated resource as a string, mirrored from the bound allocation
	// for convenient access without chasing the reference.
	// +optional
	Value string `json:"value,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=claims
// +kubebuilder:resource:singular=claim
// +kubebuilder:resource:shortName=claim
// +kubebuilder:printcolumn:name="Value",type=string,JSONPath=`.status.value`
// +kubebuilder:printcolumn:name="Allocated",type=string,JSONPath=`.status.conditions[?(@.type=="Allocated")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Claim is the Schema for the claims API
type Claim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ClaimSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ClaimStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (c *Claim) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (c *Claim) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ClaimList contains a list of Claim
type ClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Claim `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Claim{}, &ClaimList{})
		return nil
	})
}
