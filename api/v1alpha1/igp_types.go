// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=isis
type IGPType string

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=igps
// +kubebuilder:resource:singular=igp
// +kubebuilder:resource:shortName=igp
// +kubebuilder:printcolumn:name="Process name",type=string,JSONPath=`.spec.name`

// IGP is the Schema for the igp API.
type IGP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec IGPSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status IGPStatus `json:"status"`
}

// IGProcessSpec defines the desired state of Interior Gateway Protocol (IGP) process on a device.
type IGPSpec struct {
	// The name of the routing instance.
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	//+kubebuilder:validation:Optional
	ISIS *ISISSpec `json:"isis,omitempty"`
}

// IGPStatus defines the observed state of an IGP process.
type IGPStatus struct {
	// The conditions are a list of status objects that describe the state of the Interface.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true

// IGPList contains a list of IGP (processes).
type IGPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IGP `json:"items"`
}

// +kubebuilder:validation:Enum=Level1;Level2;Level12
type ISISLevel string

// +kubebuilder:validation:Enum=v4-unicast;v6-unicast
type ISISAF string

type ISISSpec struct {
	// The Network Entity Title (NET) for the ISIS instance.
	//+kubebuilder:validation:Required
	NET string `json:"net"`
	// The is-type of the process (the level)
	//+kubebuilder:validation:Required
	Level ISISLevel `json:"level"`
	// Overload bit configuration for this ISIS instance
	//+kubebuilder:validation:Optional
	OverloadBit *OverloadBit `json:"overloadBit"`
	//+kubebuilder:validation:Optional
	AddressFamilies []ISISAF `json:"addressFamilies,omitempty"`
}

type OverloadBit struct {
	// Duration of the OverloadBit in seconds.
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Minimum=576
	//+kubebuilder:validation:Maximum=86400
	OnStartup uint32 `json:"onStartup"`
}

func init() {
	SchemeBuilder.Register(&IGP{}, &IGPList{})
}
