// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VLANSpec defines the desired state of VLAN
type VLANSpec struct {
	// DeviceName is a ref to the device owning this VLAN. Device must exist in the same namespace as the VLAN.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="deviceName is immutable"
	DeviceName string `json:"device"`

	// ID is the VLAN ID.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	// +required
	ID int16 `json:"id"`

	// AdminState indicates whether the interface is administratively up or down.
	//
	// +required
	AdminState AdminState `json:"adminState"`
}

// VLANStatus defines the observed state of VLAN.
type VLANStatus struct {
	// The conditions are a list of status objects that describe the state of the VLAN.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vlans
// +kubebuilder:resource:singular=vlan
// +kubebuilder:printcolumn:name="VLAN",type=string,JSONPath=`.spec.ID`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VLAN is the Schema for the vlans API
type VLAN struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of VLAN
	// +required
	Spec VLANSpec `json:"spec"`

	// status defines the observed state of VLAN
	// +optional
	Status VLANStatus `json:"status,omitempty,omitzero"`
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
