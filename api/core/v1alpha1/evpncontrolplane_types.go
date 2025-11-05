// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EVPNControlPlaneSpec defines the desired state of EVPNControlPlane
type EVPNControlPlaneSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// VTEPRef is a reference to the VTEP associated with this EVPN Control Plane.
	// VTEPRef LocalObjectReference `json:"vtepRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// HostReachability defines the host reachability type for this EVPN Control Plane.
	// +required
	// +kubebuilder:validation:Enum=FloodAndLearn;BGP
	HostReachability HostReachabilityType `json:"hostReachability"`

	// SuppressARP indicates whether ARP suppression is enabled for this EVPN Control Plane.
	// +required
	SuppressARP bool `json:"suppressArp"`
}

type HostReachabilityType string

const (
	HostReachabilityBGP           HostReachabilityType = "BGP"
	HostReachabilityFloodAndLearn HostReachabilityType = "FloodAndLearn"
)

// EVPNControlPlaneStatus defines the observed state of EVPNControlPlane.
type EVPNControlPlaneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the EVPNControlPlane resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// EVPNControlPlane is the Schema for the evpncontrolplanes API
type EVPNControlPlane struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of EVPNControlPlane
	// +required
	Spec EVPNControlPlaneSpec `json:"spec"`

	// status defines the observed state of EVPNControlPlane
	// +optional
	Status EVPNControlPlaneStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// EVPNControlPlaneList contains a list of EVPNControlPlane
type EVPNControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EVPNControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EVPNControlPlane{}, &EVPNControlPlaneList{})
}
