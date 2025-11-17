// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VLANSpec defines the desired state of VLAN
type VLANSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this vlan.
	// This reference is used to link the VLAN to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// ID is the VLAN ID. Valid values are between 1 and 4094.
	// Immutable.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	ID int16 `json:"id"`

	// Name is the name of the VLAN.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[^\s]+$`
	Name string `json:"name,omitempty"`

	// AdminState indicates whether the VLAN is administratively active or inactive/suspended.
	// +optional
	// +kubebuilder:default=Active
	AdminState VLANState `json:"adminState"`
}

// VLANState represents the administrative state of the VLAN.
// +kubebuilder:validation:Enum=Active;Suspended
type VLANState string

const (
	// VLANStateActive indicates that the VLAN is administratively active.
	VLANStateActive VLANState = "Active"
	// VLANStateSuspended indicates that the VLAN is administratively inactive/suspended.
	VLANStateSuspended VLANState = "Suspended"
)

// VLANStatus defines the observed state of VLAN.
type VLANStatus struct {
	// The conditions are a list of status objects that describe the state of the VLAN.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// RoutedBy references the interface that provides Layer 3 routing for this VLAN, if any.
	// This field is set when an Interface of type RoutedVLAN references this VLAN.
	// +optional
	RoutedBy *LocalObjectReference `json:"routedBy,omitempty"`

	// BridgedBy references the EVPNInstance that provides a L2VNI for this VLAN, if any.
	// This field is set when an EVPNInstance of type Bridged references this VLAN.
	// +optional
	BridgedBy *LocalObjectReference `json:"bridgedBy,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vlans
// +kubebuilder:resource:singular=vlan
// +kubebuilder:printcolumn:name="VLAN-ID",type=string,JSONPath=`.spec.id`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VLAN is the Schema for the vlans API
type VLAN struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec VLANSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status VLANStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (v *VLAN) GetConditions() []metav1.Condition {
	return v.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (v *VLAN) SetConditions(conditions []metav1.Condition) {
	v.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// VLANList contains a list of VLAN
type VLANList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VLAN `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VLAN{}, &VLANList{})
}
