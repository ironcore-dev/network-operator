// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ExtCommunitySetSpec defines the desired state of ExtCommunitySet.
type ExtCommunitySetSpec struct {
	// DeviceRef is a reference to the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the ExtCommunitySet on the device.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// Members is the ordered list of extended community-list entries.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Members []CommunityMember `json:"members"`
}

// ExtCommunitySetStatus defines the observed state of ExtCommunitySet.
type ExtCommunitySetStatus struct {
	// Conditions is a list of status conditions describing the state of the ExtCommunitySet.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=extcommunitysets
// +kubebuilder:resource:singular=extcommunityset
// +kubebuilder:printcolumn:name="Ext Community Set",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ExtCommunitySet is the Schema for the extcommunitysets API.
type ExtCommunitySet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec ExtCommunitySetSpec `json:"spec,omitempty"`

	// +optional
	Status ExtCommunitySetStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (e *ExtCommunitySet) GetConditions() []metav1.Condition {
	return e.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (e *ExtCommunitySet) SetConditions(conditions []metav1.Condition) {
	e.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ExtCommunitySetList contains a list of ExtCommunitySet.
type ExtCommunitySetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ExtCommunitySet `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ExtCommunitySet{}, &ExtCommunitySetList{})
		return nil
	})
}
