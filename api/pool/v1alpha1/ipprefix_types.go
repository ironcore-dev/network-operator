// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IPPrefixSpec defines the desired state of IPPrefix.
type IPPrefixSpec struct {
	// PoolRef references the IPPrefixPool this prefix was allocated from.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="poolRef is immutable"
	PoolRef corev1alpha1.TypedLocalObjectReference `json:"poolRef"`

	// Prefix is the reserved CIDR prefix.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="prefix is immutable"
	Prefix corev1alpha1.IPPrefix `json:"prefix"`

	// ClaimRef references the Claim bound to this prefix.
	// Nil when the prefix is unbound (pre-provisioned or retained).
	// +optional
	ClaimRef *ClaimRef `json:"claimRef,omitempty"`
}

// IPPrefixStatus defines the observed state of IPPrefix.
type IPPrefixStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the IPPrefix resource.
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
// +kubebuilder:resource:path=ipprefixes,singular=ipprefix,shortName=pfx
// +kubebuilder:printcolumn:name="Pool",type=string,JSONPath=`.spec.poolRef.name`
// +kubebuilder:printcolumn:name="Prefix",type=string,JSONPath=`.spec.prefix`
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.spec.claimRef.name`
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type=="Valid")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// IPPrefix is the Schema for the ipprefixes API.
type IPPrefix struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IPPrefixSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IPPrefixStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (p *IPPrefix) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IPPrefix) SetConditions(conds []metav1.Condition) {
	p.Status.Conditions = conds
}

// ClaimRef returns the ClaimRef bound to this allocation, or nil if unbound.
func (p *IPPrefix) ClaimRef() *ClaimRef {
	return p.Spec.ClaimRef
}

// SetClaimRef sets or clears the ClaimRef on this allocation.
func (p *IPPrefix) SetClaimRef(ref *ClaimRef) {
	p.Spec.ClaimRef = ref
}

// Value returns the allocated value as a string.
func (p *IPPrefix) Value() string {
	return p.Spec.Prefix.String()
}

// +kubebuilder:object:root=true

// IPPrefixList contains a list of IPPrefix.
type IPPrefixList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPPrefix `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IPPrefix{}, &IPPrefixList{})
		return nil
	})
}
