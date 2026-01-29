// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AccessControlListSpec defines the desired state of AccessControlList
type AccessControlListSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the AccessControlList to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the identifier of the AccessControlList on the device.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// A list of rules/entries to apply.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Entries []ACLEntry `json:"entries"`
}

type ACLEntry struct {
	// The sequence number of the ACL entry.
	// +required
	// +kubebuilder:validation:Minimum=1
	Sequence int32 `json:"sequence"`

	// The forwarding action of the ACL entry.
	// +required
	Action ACLAction `json:"action"`

	// The protocol to match. If not specified, defaults to "IP".
	// Available options are: ICMP, IP, OSPF, PIM, TCP, UDP.
	// +optional
	// +kubebuilder:default=IP
	Protocol Protocol `json:"protocol"`

	// Source IP address prefix. Can be IPv4 or IPv6.
	// Use 0.0.0.0/0 (::/0) to represent 'any'.
	// +required
	SourceAddress IPPrefix `json:"sourceAddress"`

	// Destination IP address prefix. Can be IPv4 or IPv6.
	// Use 0.0.0.0/0 (::/0) to represent 'any'.
	// +required
	DestinationAddress IPPrefix `json:"destinationAddress"`

	// Description provides a human-readable description of the ACL entry.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Description string `json:"description,omitempty"`
}

// Protocol represents the protocol type for an ACL entry.
// +kubebuilder:validation:Enum=ICMP;IP;OSPF;PIM;TCP;UDP
type Protocol string

const (
	ProtocolICMP Protocol = "ICMP"
	ProtocolIP   Protocol = "IP"
	ProtocolOSPF Protocol = "OSPF"
	ProtocolPIM  Protocol = "PIM"
	ProtocolTCP  Protocol = "TCP"
	ProtocolUDP  Protocol = "UDP"
)

// ACLAction represents the type of action that can be taken by an ACL rule.
// +kubebuilder:validation:Enum=Permit;Deny
type ACLAction string

const (
	// ActionPermit allows traffic that matches the rule.
	ActionPermit ACLAction = "Permit"
	// ActionDeny blocks traffic that matches the rule.
	ActionDeny ACLAction = "Deny"
)

// AccessControlListStatus defines the observed state of AccessControlList.
type AccessControlListStatus struct {
	// EntriesSummary provides a human-readable summary of the number of ACL entries.
	// +optional
	EntriesSummary string `json:"entriesSummary,omitempty"`

	// The conditions are a list of status objects that describe the state of the AccessControlList.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=accesscontrollists
// +kubebuilder:resource:singular=accesscontrollist
// +kubebuilder:resource:shortName=acl
// +kubebuilder:printcolumn:name="ACL",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Entries",type=string,JSONPath=`.status.entriesSummary`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AccessControlList is the Schema for the accesscontrollists API
type AccessControlList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec AccessControlListSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status AccessControlListStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (acl *AccessControlList) GetConditions() []metav1.Condition {
	return acl.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (acl *AccessControlList) SetConditions(conditions []metav1.Condition) {
	acl.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// AccessControlListList contains a list of AccessControlList
type AccessControlListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessControlList `json:"items"`
}

var (
	AccessControlListDependencies   []schema.GroupVersionKind
	accessControlListDependenciesMu sync.Mutex
)

func RegisterAccessControlListDependency(gvk schema.GroupVersionKind) {
	accessControlListDependenciesMu.Lock()
	defer accessControlListDependenciesMu.Unlock()
	AccessControlListDependencies = append(AccessControlListDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&AccessControlList{}, &AccessControlListList{})
}
