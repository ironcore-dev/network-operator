// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IPAddressSpec defines the desired state of IPAddress.
type IPAddressSpec struct {
	// PoolRef references the IPAddressPool this address was allocated from.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="poolRef is immutable"
	PoolRef corev1alpha1.TypedLocalObjectReference `json:"poolRef"`

	// Address is the reserved IP address.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="address is immutable"
	Address corev1alpha1.IPAddr `json:"address"`

	// ClaimRef references the Claim bound to this address.
	// Nil when the address is unbound (pre-provisioned or retained).
	// +optional
	ClaimRef *ClaimRef `json:"claimRef,omitempty"`
}

// IPAddressStatus defines the observed state of IPAddress.
type IPAddressStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the IPAddress resource.
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
// +kubebuilder:resource:path=ipaddresses,singular=ipaddress,shortName=ipa
// +kubebuilder:printcolumn:name="Pool",type=string,JSONPath=`.spec.poolRef.name`
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.spec.address`
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.spec.claimRef.name`
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type=="Valid")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// IPAddress is the Schema for the ipaddresses API.
type IPAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IPAddressSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IPAddressStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (a *IPAddress) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (a *IPAddress) SetConditions(conds []metav1.Condition) {
	a.Status.Conditions = conds
}

// ClaimRef returns the ClaimRef bound to this allocation, or nil if unbound.
func (a *IPAddress) ClaimRef() *ClaimRef {
	return a.Spec.ClaimRef
}

// SetClaimRef sets or clears the ClaimRef on this allocation.
func (a *IPAddress) SetClaimRef(ref *ClaimRef) {
	a.Spec.ClaimRef = ref
}

// Value returns the allocated value as a string.
func (a *IPAddress) Value() string {
	return a.Spec.Address.String()
}

// +kubebuilder:object:root=true

// IPAddressList contains a list of IPAddress.
type IPAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IPAddress{}, &IPAddressList{})
		return nil
	})
}
