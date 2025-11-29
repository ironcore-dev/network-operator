// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrefixSetSpec defines the desired state of PrefixSet
type PrefixSetSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Banner to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the PrefixSet.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// A list of entries to apply.
	// The address families (IPv4, IPv6) of all prefixes in the list must match.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Entries []PrefixEntry `json:"entries"`
}

type PrefixEntry struct {
	// The sequence number of the Prefix entry.
	// +required
	// +kubebuilder:validation:Minimum=1
	Sequence int32 `json:"sequence"`

	// IP prefix. Can be IPv4 or IPv6.
	// Use 0.0.0.0/0 (::/0) to represent 'any'.
	// +required
	Prefix IPPrefix `json:"prefix"`

	// Optional mask length range for the prefix.
	// If not specified, only the exact prefix length is matched.
	// +optional
	MaskLengthRange *MaskLengthRange `json:"maskLengthRange,omitempty"`
}

type MaskLengthRange struct {
	// Minimum mask length.
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=128
	Min int8 `json:"min"`

	// Maximum mask length.
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=128
	Max int8 `json:"max"`
}

// PrefixSetStatus defines the observed state of PrefixSet.
type PrefixSetStatus struct {
	// The conditions are a list of status objects that describe the state of the PrefixSet.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=prefixsets
// +kubebuilder:resource:singular=prefixset
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PrefixSet is the Schema for the prefixsets API
type PrefixSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec PrefixSetSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status PrefixSetStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (p *PrefixSet) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *PrefixSet) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// Is4 reports whether entries of the PrefixSet are IPv4 addresses.
func (p *PrefixSet) Is4() bool {
	// Note: We can safely check only the first entry because
	// validation ensures all entries are of the same IP family.
	return len(p.Spec.Entries) > 0 && p.Spec.Entries[0].Prefix.Addr().Is4()
}

// Is6 reports whether entries of the PrefixSet are IPv6 addresses.
func (p *PrefixSet) Is6() bool {
	// Note: We can safely check only the first entry because
	// validation ensures all entries are of the same IP family.
	return len(p.Spec.Entries) > 0 && p.Spec.Entries[0].Prefix.Addr().Is6()
}

// +kubebuilder:object:root=true

// PrefixSetList contains a list of PrefixSet
type PrefixSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PrefixSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrefixSet{}, &PrefixSetList{})
}
