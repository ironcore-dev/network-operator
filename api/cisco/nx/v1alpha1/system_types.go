// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SystemSpec defines the desired state of System
type SystemSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef v1alpha1.LocalObjectReference `json:"deviceRef"`

	// JumboMtu defines the system-wide jumbo MTU setting.
	// Valid values are from 1501 to 9216.
	// +optional
	// +kubebuilder:validation:Minimum=1501
	// +kubebuilder:validation:Maximum=9216
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=9216
	JumboMTU int16 `json:"jumboMtu"`

	// ReservedVlan specifies the VLAN ID to be reserved for system use.
	// Valid values are from 1 to 4032.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4032
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=3968
	ReservedVlan int16 `json:"reservedVlan"`

	// VlanLongName enables or disables 128-character VLAN names
	// Disabled by default.
	// +optional
	// +kubebuilder:default=false
	VlanLongName bool `json:"vlanLongName"`
}

// SystemStatus defines the observed state of System.
type SystemStatus struct {
	// The conditions are a list of status objects that describe the state of the Banner.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=systems
// +kubebuilder:resource:singular=system
// +kubebuilder:resource:shortName=nxsystem
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// System is the Schema for the systems API
type System struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec SystemSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status SystemStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (s *System) GetConditions() []metav1.Condition {
	return s.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (s *System) SetConditions(conditions []metav1.Condition) {
	s.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// SystemList contains a list of System
type SystemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []System `json:"items"`
}

func init() {
	SchemeBuilder.Register(&System{}, &SystemList{})
}
