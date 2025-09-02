// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SNMPSpec defines the desired state of SNMP
type SNMPSpec struct {
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

	// The contact information for the SNMP server.
	// +optional
	Contact string `json:"contact"`

	// The location information for the SNMP server.
	// +optional
	Location string `json:"location"`

	// The name of the interface to be used for sending out SNMP Trap/Inform notifications.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	SourceInterfaceName string `json:"sourceInterfaceName"`

	// SNMP communities for SNMPv1 or SNMPv2c.
	// +optional
	Communities []SNMPCommunity `json:"communities,omitempty"`

	// SNMP destination hosts for SNMP traps or informs messages.
	// +required
	// +kubebuilder:validation:MinItems=1
	Hosts []SNMPHosts `json:"hosts"`

	// The list of trap notifications to enable.
	// +optional
	Traps []string `json:"traps,omitempty"`
}

type SNMPCommunity struct {
	// Name of the community.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Group to which the community belongs.
	// +optional
	Group string `json:"group,omitempty"`

	// ACL name to filter SNMP requests.
	// +optional
	ACLName string `json:"aclName,omitempty"`
}

type SNMPHosts struct {
	// The Hostname or IP address of the SNMP host to send notifications to.
	// +kubebuilder:validation:MinLength=1
	// +required
	Address string `json:"address"`

	// Type of message to send to host. Default is traps.
	// +kubebuilder:validation:Enum=Traps;Informs
	// +kubebuilder:default=Traps
	// +optional
	Type string `json:"type"`

	// SNMP version. Default is v2c.
	// +kubebuilder:validation:Enum=v1;v2c;v3
	// +kubebuilder:default=v2c
	// +optional
	Version string `json:"version"`

	// SNMP community or user name.
	// +optional
	Community string `json:"community,omitempty"`

	// The name of the vrf instance to use to source traffic.
	// +optional
	VrfName string `json:"vrfName,omitempty"`
}

// SNMPStatus defines the observed state of SNMP.
type SNMPStatus struct {
	// The conditions are a list of status objects that describe the state of the SNMP.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=snmp
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SNMP is the Schema for the snmp API
type SNMP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec SNMPSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status SNMPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SNMPList contains a list of SNMP
type SNMPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SNMP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SNMP{}, &SNMPList{})
}
