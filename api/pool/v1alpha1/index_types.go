// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IndexSpec defines the desired state of Index.
type IndexSpec struct {
	// PoolRef references the IndexPool this index was allocated from.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="poolRef is immutable"
	PoolRef corev1alpha1.TypedLocalObjectReference `json:"poolRef"`

	// Index is the reserved value.
	// Immutable.
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="index is immutable"
	Index int64 `json:"index"`

	// ClaimRef references the Claim bound to this index.
	// Nil when the index is unbound (pre-provisioned or retained).
	// +optional
	ClaimRef *ClaimRef `json:"claimRef,omitempty"`
}

// IndexStatus defines the observed state of Index.
type IndexStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Index resource.
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
// +kubebuilder:resource:path=indices,singular=index,shortName=idx
// +kubebuilder:printcolumn:name="Pool",type=string,JSONPath=`.spec.poolRef.name`
// +kubebuilder:printcolumn:name="Index",type=string,JSONPath=`.spec.index`
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.spec.claimRef.name`
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type=="Valid")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Index is the Schema for the indices API.
type Index struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IndexSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IndexStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (i *Index) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (i *Index) SetConditions(conds []metav1.Condition) {
	i.Status.Conditions = conds
}

// ClaimRef returns the ClaimRef bound to this allocation, or nil if unbound.
func (i *Index) ClaimRef() *ClaimRef {
	return i.Spec.ClaimRef
}

// SetClaimRef sets or clears the ClaimRef on this allocation.
func (i *Index) SetClaimRef(ref *ClaimRef) {
	i.Spec.ClaimRef = ref
}

// Value returns the allocated value as a string.
func (i *Index) Value() string {
	return strconv.FormatInt(i.Spec.Index, 10)
}

// +kubebuilder:object:root=true

// IndexList contains a list of Index.
type IndexList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Index `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Index{}, &IndexList{})
		return nil
	})
}
