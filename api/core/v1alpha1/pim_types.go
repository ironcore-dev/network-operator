// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PIMSpec defines the desired state of PIM
type PIMSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the PIM to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// RendezvousPoints defines the list of rendezvous points for sparse mode multicast.
	// +optional
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MinItems=1
	RendezvousPoints []RendezvousPoint `json:"rendezvousPoints,omitempty"`

	// InterfaceRefs is a list of interfaces that are part of the PIM instance.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	InterfaceRefs []PIMInterface `json:"interfaceRefs,omitempty"`
}

type RendezvousPoint struct {
	// Address is the IPv4 address of the rendezvous point.
	// +required
	// +kubebuilder:validation:Format=ipv4
	Address string `json:"address"`

	// MulticastGroups defined the list of multicast IPv4 address ranges associated with the rendezvous point.
	// If not specified, the rendezvous point will be used for all multicast groups.
	// +optional
	MulticastGroups []IPPrefix `json:"multicastGroups,omitempty"`

	// AnycastAddresses is a list of redundant anycast ipv4 addresses associated with the rendezvous point.
	// +optional
	// +listType=set
	// +kubebuilder:validation:items:Format=ipv4
	AnycastAddresses []string `json:"anycastAddresses,omitempty"`
}

type PIMInterface struct {
	LocalObjectReference `json:",inline"`

	// Mode is the PIM mode to use when delivering multicast traffic via this interface.
	// +optional
	// +kubebuilder:default=Sparse
	Mode PIMInterfaceMode `json:"mode"`
}

// PIMInterfaceMode represents the mode of a PIM interface.
// +kubebuilder:validation:Enum=Sparse;Dense
type PIMInterfaceMode string

const (
	PIMModeSparse PIMInterfaceMode = "Sparse"
	PIMModeDense  PIMInterfaceMode = "Dense"
)

// PIMStatus defines the observed state of PIM.
type PIMStatus struct {
	// The conditions are a list of status objects that describe the state of the PIM.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=pim
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PIM is the Schema for the pim API
type PIM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec PIMSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status PIMStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (pim *PIM) GetConditions() []metav1.Condition {
	return pim.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (pim *PIM) SetConditions(conditions []metav1.Condition) {
	pim.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// PIMList contains a list of PIM
type PIMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PIM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PIM{}, &PIMList{})
}
