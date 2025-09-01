// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AccessControlListSpec defines the desired state of AccessControlList
type AccessControlListSpec struct {
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

	// Name is the name of the interface.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name"`

	// A list of rules/entries to apply.
	// +required
	// +kubebuilder:validation:MinItems=1
	Entries []ACLEntry `json:"entries"`
}

type ACLEntry struct {
	// The sequence number of the ACL entry.
	// +required
	Sequence int `json:"sequence"`

	// The forwarding action of the ACL entry.
	// +required
	Action ACLAction `json:"action"`

	// The protocol to match. If not specified, defaults to "ip".
	// +kubebuilder:validation:Enum=icmp;ip;ospf;pim;tcp;udp
	// +kubebuilder:default=ip
	// +optional
	Protocol string `json:"protocol,omitempty"`

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
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`
}

func (e *ACLEntry) Validate() error {
	if !e.SourceAddress.IsValid() {
		return fmt.Errorf("invalid IP prefix: %s", e.SourceAddress.String())
	}
	if !e.DestinationAddress.IsValid() {
		return fmt.Errorf("invalid IP prefix: %s", e.SourceAddress.String())
	}
	return nil
}

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
	// The conditions are a list of status objects that describe the state of the AccessControlList.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=accesscontrollists
// +kubebuilder:resource:singular=accesscontrollist
// +kubebuilder:resource:shortName=acl
// +kubebuilder:printcolumn:name="ACL",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AccessControlList is the Schema for the accesscontrollists API
type AccessControlList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec AccessControlListSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status AccessControlListStatus `json:"status,omitempty,omitzero"`
}

func (acl *AccessControlList) Validate() error {
	set := map[int]struct{}{}
	for _, entry := range acl.Spec.Entries {
		if _, exists := set[entry.Sequence]; exists {
			return fmt.Errorf("duplicate sequence number %d in ACL %q", entry.Sequence, acl.Name)
		}
		set[entry.Sequence] = struct{}{}
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("invalid entry in acl %q: %w", acl.Name, err)
		}
	}
	return nil
}

// +kubebuilder:object:root=true

// AccessControlListList contains a list of AccessControlList
type AccessControlListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessControlList `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessControlList{}, &AccessControlListList{})
}
