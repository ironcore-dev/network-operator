// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VRFSpec defines the desired state of VRF
type VRFSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the VRF to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the VRF.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// Description provides a human-readable description of the VRF.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`

	// VNI is the VXLAN Network Identifier for the VRF (always an L3).
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16777215
	VNI uint32 `json:"vni,omitempty"`

	// RouteDistinguisher is the route distinguisher for the VRF.
	// Formats supported:
	//  - Type 0: <asn(0-65535)>:<number(0-4294967295)>
	//  - Type 1: <ipv4>:<number(0-65535)>
	//  - Type 2: <asn(65536-4294967295)>:<number(0-65535)>
	//
	// Validation via admission webhook for the VRF type.
	//
	// +optional
	RouteDistinguisher string `json:"routeDistinguisher,omitempty"`

	// RouteTargets is the list of route targets for the VRF.
	// +optional
	// +listType=map
	// +listMapKey=value
	RouteTargets []RouteTarget `json:"routeTargets,omitempty"`
}

// RouteTargetAF represents a supported address family value.
// +kubebuilder:validation:Enum=IPv4;IPv6;IPv4EVPN;IPv6EVPN
type RouteTargetAF string

const (
	IPv4     RouteTargetAF = "IPv4"
	IPv6     RouteTargetAF = "IPv6"
	IPv4EVPN RouteTargetAF = "IPv4EVPN"
	IPv6EVPN RouteTargetAF = "IPv6EVPN"
)

// RouteTargetAction represents the action for a route target.
// +kubebuilder:validation:Enum=Import;Export;Both
type RouteTargetAction string

const (
	RouteTargetActionImport RouteTargetAction = "Import"
	RouteTargetActionExport RouteTargetAction = "Export"
	RouteTargetActionBoth   RouteTargetAction = "Both"
)

type RouteTarget struct {
	// Value is the route target value, must have the format as VRFSpec.RouteDistinguisher. Validation via
	// admission webhook.
	//
	// +required
	Value string `json:"value"`

	// AddressFamilies is the list of address families for the route target.
	// +required
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	AddressFamilies []RouteTargetAF `json:"addressFamilies,omitempty"`

	// Action defines whether the route target is imported, exported, or both
	// +required
	Action RouteTargetAction `json:"action"`
}

// VRFStatus defines the observed state of VRF.
type VRFStatus struct {
	// The conditions are a list of status objects that describe the state of the VRF.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vrfs
// +kubebuilder:resource:singular=vrf
// +kubebuilder:printcolumn:name="VRF",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VRF is the Schema for the vrfs API
// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-vrf,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=vrfs,verbs=create;update,versions=v1alpha1,name=vvrf.kb.io,admissionReviewVersions=v1
type VRF struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of VRF
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec VRFSpec `json:"spec"`

	// status of the resource. This is set and updated automatically.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status VRFStatus `json:"status,omitempty,omitzero"`
}

// GetConditions returns the list of conditions for the VRF.
func (v *VRF) GetConditions() []metav1.Condition {
	return v.Status.Conditions
}

// SetConditions sets the conditions for the VRF.
func (v *VRF) SetConditions(conditions []metav1.Condition) {
	v.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// VRFList contains a list of VRF
type VRFList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VRF `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VRF{}, &VRFList{})
}
