// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// / ConfidentialityOffset represents the number of octets in an Ethernet frame that are sent in unencrypted plain-text.
type ConfidentialityOffset uint8

// +kubebuilder:validation:Enum=0;30;50
const (
	// ZeroBytes represents broadcast traffic.
	ZeroBytes ConfidentialityOffset = 0
	// ThirtyBytes represents multicast traffic.
	ThirtyBytes ConfidentialityOffset = 30
	// FiftyBytes represents unicast traffic.
	FiftyBytes ConfidentialityOffset = 50
)

// MacSecSpec defines the desired state of MacSec.
type MacSecSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the interface.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// Description provides a human-readable description of the macsec policy.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`

	// InterfaceRef is a reference to the interface on which MACsec is enabled. The interface must exist on the Device specified by deviceRef.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="InterfaceRef is immutable"
	InterfaceRef LocalObjectReference `json:"interfaceRef,omitempty"`

	// PreSharedKeyRef is a list of references to pre-shared keys used for MACSec encryption.
	// +optional
	PreSharedKeyRef []LocalObjectReference `json:"preSharedkeyRef,omitempty"`

	// Policy defines the MACSec policy configuration.
	// +optional
	Policy *MacSecPolicy `json:"policy,omitempty"`
}

type MacSecPolicy struct {
	CipherSuite string `json:"cipherSuite,omitempty"`

	// Number of octets in an Ethernet frame that are sent in unencrypted plain-text
	// +optional
	ConfidentialityOffset ConfidentialityOffset `json:"confidentialityOffset,omitempty"`

	// Specifies the key server priority used by the MACsec Key Agreement (MKA) protocol to select the key server when
	// MACsec is enabled using static connectivity association key (CAK) security mode.
	// +optional
	KeyServerPriority uint8 `json:"keyServerPriority,omitempty"`

	// Number of out-of-order packets that can be received before the secure channel is considered to be inoperable.
	// A value of 0 means that frames are accepted only in the correct order.
	// +optional
	RelayProtection uint16 `json:"relayProtection,omitempty"`
}

// MacSecStatus defines the observed state of MacSec.
type MacSecStatus struct {
	// The conditions are a list of status objects that describe the state of the MacSec.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Device",type="string",JSONPath=".spec.deviceRef.name",description="The device this MacSec is configured on"
// +kubebuilder:printcolumn:name="Interface",type="string",JSONPath=".spec.interfaceRef.name",description="The interface this MacSec is configured on"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Ready status of the MacSec"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MacSec is the Schema for the macsecs API.
type MacSec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MacSecSpec   `json:"spec,omitempty"`
	Status MacSecStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MacSecList contains a list of MacSec.
type MacSecList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MacSec `json:"items"`
}

// GetConditions implements conditions.Getter.
func (m *MacSec) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (m *MacSec) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&MacSec{}, &MacSecList{})
}
