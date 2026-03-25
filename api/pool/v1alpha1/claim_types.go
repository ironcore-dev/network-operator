// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// ClaimSpec defines the desired state of Claim
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

	// Allocation describes the resource reserved for this claim.
	// +optional
	Allocation *ClaimAllocation `json:"allocation,omitempty"`
}

// ClaimAllocation holds the allocated resource value for a claim.
// +kubebuilder:validation:XValidation:rule="[has(self.index), has(self.ipAddress), has(self.prefix)].filter(x, x).size() == 1",message="exactly one allocation field must be set"
type ClaimAllocation struct {
	// Index is set when the allocation is sourced from an IndexPool.
	// +optional
	Index *uint64 `json:"index,omitempty"`

	// IPAddress is set when the allocation is sourced from an IPAddressPool.
	// +optional
	IPAddress *string `json:"ipAddress,omitempty"`

	// Prefix is set when the allocation is sourced from an IPPrefixPool.
	// +optional
	Prefix *corev1alpha1.IPPrefix `json:"prefix,omitempty"`

	// Value is the string representation of the allocated resource.
	// +optional
	Value string `json:"value,omitempty"`
}

// String implements [fmt.Stringer].
func (a *ClaimAllocation) String() string {
	if a == nil {
		return ""
	}
	switch {
	case a.Index != nil:
		return strconv.FormatUint(*a.Index, 10)
	case a.IPAddress != nil:
		return *a.IPAddress
	case a.Prefix != nil:
		return a.Prefix.String()
	default:
		return ""
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=claims
// +kubebuilder:resource:singular=claim
// +kubebuilder:resource:shortName=claim
// +kubebuilder:printcolumn:name="Value",type=string,JSONPath=`.status.allocation.value`
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
