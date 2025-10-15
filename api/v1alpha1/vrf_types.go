// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VRFSpec defines the basic structure of a Virtual Routing and Forwarding (VRF) instance/context. A VRF has one single Route
// Distinguisher (RD) and a set of Route Targets (RTs), which can be empty. RTs define how the VRF learns about extra routes.
//
// References:
// [RFC 4364] https://datatracker.ietf.org/doc/html/rfc4364 - BGP/MPLS IP Virtual Private Networks (VPNs)
// [RFC 7432] https://datatracker.ietf.org/doc/html/rfc7432 - BGP MPLS-Based Ethernet VPN
type VRFSpec struct {
	// DeviceName is a ref to the device owning this VRF. Device must exist in the same namespace as the VRF.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="deviceName is immutable"
	DeviceName string `json:"deviceName"`

	// Name defines the name of the VRF.
	//
	// +required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]+$`
	Name string `json:"name"`

	// AdminState defines the administrative state of the VRF.
	//
	// +required
	AdminState AdminState `json:"adminState"`

	// RouteDistinguisher is the route distinguisher as per [RFC 4364]
	//
	// +required
	// +kubebuilder:validation:Pattern=`^([0-9]+:[0-9]+|([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+)$`
	RouteDistinguisher string `json:"routeDistinguisher"`

	// RouteTargets defines the list of route targets associated with the VRF.
	//
	// +optional
	RouteTargets []RouteTarget `json:"routeTargets,omitempty"`
}

// VRFStatus defines the observed state of VRF.
type VRFStatus struct {
	// The conditions are a list of status objects that describe the state of the VRF.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vrfs
// +kubebuilder:resource:singular=vrf
// +kubebuilder:printcolumn:name="VRF",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//
// VRF is the Schema for the VRFs API
type VRF struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	Spec   VRFSpec   `json:"spec,omitempty"`
	Status VRFStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VRFList contains a list of VRF
type VRFList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VRF `json:"items"`
}

// RouteTarget defines the structure for a single route target in a VRF. A RT is essentially an extended BGP
// community used to control route import/export between VRFs. RTs may be also be associated with address
// families (IPv4, IPv6), and might be also added to the EVPN via the ipv4-evpn and ipv6-evpn address families.
type RouteTarget struct {
	// Value specifies the Route Target (RT) value as per [RFC 4364].
	//
	// // +required
	// +kubebuilder:validation:Pattern=`^([0-9]+:[0-9]+|([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]+)$`
	Value string `json:"value"`

	// AddressFamily specifies the address families for the Route Target. Families ipv4-evpn and ipv6-evpn indicate
	// that the RT should be applied to the EVPN contexts (see [RFC 7432]).
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	AddressFamilies []VRFAddressFamily `json:"addressFamilies"`

	// Action specifies how this RT should be used within the VRF.
	//
	// +required
	// +kubebuilder:validation:Enum=None;Import;Export;Both
	Action RTAction `json:"action"`
}

// +kubebuilder:validation:Enum=ipv4;ipv6;ipv4-evpn;ipv6-evpn
type VRFAddressFamily string

const (
	AddressFamilyIPv4     VRFAddressFamily = "ipv4"
	AddressFamilyIPv6     VRFAddressFamily = "ipv6"
	AddressFamilyIPv4EVPN VRFAddressFamily = "ipv4-evpn"
	AddressFamilyIPv6EVPN VRFAddressFamily = "ipv6-evpn"
)

type RTAction string

const (
	RTNone   RTAction = "none"
	RTImport RTAction = "import"
	RTExport RTAction = "export"
	RTBoth   RTAction = "both"
)

func init() {
	SchemeBuilder.Register(&VRF{}, &VRFList{})
}
