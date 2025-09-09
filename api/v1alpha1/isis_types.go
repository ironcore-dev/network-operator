// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ISISSpec defines the desired state of ISIS
type ISISSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	DeviceName string `json:"deviceName,omitempty"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the interface to its provider-specific configuration.
	// Immutable.
	// +optional
	ProviderConfigRef *ProviderConfigReference `json:"providerConfigRef,omitempty"`

	// Instance is the name of the ISIS instance.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +required
	Instance string `json:"instance"`

	// NetworkEntityTitle is the NET of the ISIS instance.
	// +kubebuilder:validation:Pattern=`^[a-fA-F0-9]{2}(\.[a-fA-F0-9]{4}){3,9}\.[a-fA-F0-9]{2}$`
	// +required
	NetworkEntityTitle string `json:"networkEntityTitle"`

	// Type indicates the level of the ISIS instance.
	// +required
	Type ISISLevel `json:"type"`

	// OverloadBit indicates the overload bit of the ISIS instance.
	// +kubebuilder:default=Never
	// +optional
	OverloadBit OverloadBit `json:"overloadBit,omitempty"`

	// AddressFamilies is a list of address families for the ISIS instance.
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	// +required
	AddressFamilies []AddressFamily `json:"addressFamilies"`

	// Interfaces is a list of interfaces that are part of the ISIS instance.
	// +listType=atomic
	// +optional
	Interfaces []ISISInterface `json:"interfaces,omitempty"`
}

// ISISLevel represents the level of an ISIS instance.
//
// +kubebuilder:validation:Enum=Level1;Level2;Level1-2
type ISISLevel string

const (
	ISISLevel1  ISISLevel = "Level1"
	ISISLevel2  ISISLevel = "Level2"
	ISISLevel12 ISISLevel = "Level1-2"
)

// OverloadBit represents the overload bit of an ISIS instance.
//
// +kubebuilder:validation:Enum=Always;Never;OnStartup
type OverloadBit string

const (
	OverloadBitAlways    OverloadBit = "Always"
	OverloadBitNever     OverloadBit = "Never"
	OverloadBitOnStartup OverloadBit = "OnStartup"
)

// AddressFamily represents the address family of an ISIS instance.
//
// +kubebuilder:validation:Enum=IPv4Unicast;IPv6Unicast
type AddressFamily string

const (
	AddressFamilyIPv4Unicast AddressFamily = "IPv4Unicast"
	AddressFamilyIPv6Unicast AddressFamily = "IPv6Unicast"
)

type ISISInterface struct {
	// Ref is a reference to the interface object.
	// The interface object must exist in the same namespace.
	// +required
	Ref corev1.LocalObjectReference `json:"ref"`

	// BFD contains BFD configuration for the interface.
	// +kubebuilder:default={}
	// +optional
	BFD ISISBFD `json:"bfd,omitzero"`
}

type ISISBFD struct {
	// Enabled indicates whether BFD is enabled on the interface.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// ISISStatus defines the observed state of ISIS.
type ISISStatus struct {
	// The conditions are a list of status objects that describe the state of the ISIS.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	CurrentAdjacencies  *int32 `json:"currentAdjacencies,omitempty"`
	ExpectedAdjacencies *int32 `json:"expectedAdjacencies,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=isis
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Current",type=string,JSONPath=`.status.currentAdjacencies`
// +kubebuilder:printcolumn:name="Desired",type=string,JSONPath=`.status.expectedAdjacencies`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ISIS is the Schema for the isis API
type ISIS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ISISSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ISISStatus `json:"status,omitempty"`
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
