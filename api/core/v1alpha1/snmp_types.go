// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SNMPSpec defines the desired state of SNMP
type SNMPSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the SNMP to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// The contact information for the SNMP server.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Contact string `json:"contact,omitempty"`

	// The location information for the SNMP server.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Location string `json:"location,omitempty"`

	// The name of the interface to be used for sending out SNMP Trap/Inform notifications.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	SourceInterfaceName string `json:"sourceInterfaceName"`

	// SNMP communities for SNMPv1 or SNMPv2c.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Communities []SNMPCommunity `json:"communities,omitempty"`

	// SNMP destination hosts for SNMP traps or informs messages.
	// +required
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Hosts []SNMPHosts `json:"hosts"`

	// The list of trap notifications to enable.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	Traps []string `json:"traps,omitempty"`
}

type SNMPCommunity struct {
	// Name of the community.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Group to which the community belongs.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Group string `json:"group,omitempty"`

	// ACL name to filter SNMP requests.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	ACLName string `json:"aclName,omitempty"`
}

type SNMPHosts struct {
	// The Hostname or IP address of the SNMP host to send notifications to.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Address string `json:"address"`

	// Type of message to send to host. Default is traps.
	// +optional
	// +kubebuilder:default=Traps
	// +kubebuilder:validation:Enum=Traps;Informs
	Type string `json:"type"`

	// SNMP version. Default is v2c.
	// +optional
	// +kubebuilder:default=v2c
	// +kubebuilder:validation:Enum=v1;v2c;v3
	Version string `json:"version"`

	// SNMP community or user name.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Community string `json:"community,omitempty"`

	// The name of the vrf instance to use to source traffic.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
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
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=snmp
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
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
	Status SNMPStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (snmp *SNMP) GetConditions() []metav1.Condition {
	return snmp.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (snmp *SNMP) SetConditions(conditions []metav1.Condition) {
	snmp.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// SNMPList contains a list of SNMP
type SNMPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SNMP `json:"items"`
}

var (
	SNMPDependencies   []schema.GroupVersionKind
	snmpDependenciesMu sync.Mutex
)

func RegisterSNMPDependency(gvk schema.GroupVersionKind) {
	snmpDependenciesMu.Lock()
	defer snmpDependenciesMu.Unlock()
	SNMPDependencies = append(SNMPDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&SNMP{}, &SNMPList{})
}
