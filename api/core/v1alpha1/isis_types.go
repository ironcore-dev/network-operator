// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ISISSpec defines the desired state of ISIS
type ISISSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether the ISIS instance is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// Instance is the name of the ISIS instance.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Instance is immutable"
	Instance string `json:"instance"`

	// NetworkEntityTitle is the NET of the ISIS instance.
	// +required
	// +kubebuilder:validation:Pattern=`^[a-fA-F0-9]{2}(\.[a-fA-F0-9]{4}){3,9}\.[a-fA-F0-9]{2}$`
	NetworkEntityTitle string `json:"networkEntityTitle"`

	// Type indicates the level of the ISIS instance.
	// +required
	Type ISISLevel `json:"type"`

	// OverloadBit indicates the overload bit of the ISIS instance.
	// +optional
	// +kubebuilder:default=Never
	OverloadBit OverloadBit `json:"overloadBit,omitempty"`

	// AddressFamilies is a list of address families for the ISIS instance.
	// +required
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	AddressFamilies []AddressFamily `json:"addressFamilies"`

	// InterfaceRefs is a list of interfaces that are part of the ISIS instance.
	// +optional
	// +listType=atomic
	InterfaceRefs []LocalObjectReference `json:"interfaceRefs,omitempty"`
}

// ISISLevel represents the level of an ISIS instance.
// +kubebuilder:validation:Enum=Level1;Level2;Level1-2
type ISISLevel string

const (
	ISISLevel1  ISISLevel = "Level1"
	ISISLevel2  ISISLevel = "Level2"
	ISISLevel12 ISISLevel = "Level1-2"
)

// OverloadBit represents the overload bit of an ISIS instance.
// +kubebuilder:validation:Enum=Always;Never;OnStartup
type OverloadBit string

const (
	OverloadBitAlways    OverloadBit = "Always"
	OverloadBitNever     OverloadBit = "Never"
	OverloadBitOnStartup OverloadBit = "OnStartup"
)

// AddressFamily represents the address family of an ISIS instance.
// +kubebuilder:validation:Enum=IPv4Unicast;IPv6Unicast
type AddressFamily string

const (
	AddressFamilyIPv4Unicast AddressFamily = "IPv4Unicast"
	AddressFamilyIPv6Unicast AddressFamily = "IPv6Unicast"
)

// ISISStatus defines the observed state of ISIS.
type ISISStatus struct {
	// The conditions are a list of status objects that describe the state of the ISIS.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=isis
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ISIS is the Schema for the isis API
type ISIS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ISISSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ISISStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (isis *ISIS) GetConditions() []metav1.Condition {
	return isis.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (isis *ISIS) SetConditions(conditions []metav1.Condition) {
	isis.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ISISList contains a list of ISIS
type ISISList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ISIS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ISIS{}, &ISISList{})
}
