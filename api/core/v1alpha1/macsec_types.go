// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// InterfaceRef is a reference to the interface on which MACsec is enabled. The interface must exist on the Device specified by deviceRef.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="InterfaceRef is immutable"
	InterfaceRef LocalObjectReference `json:"interfaceRef,omitempty"`

	// PreSharedKeys is a list of pre-shared keys used for MACSec encryption.
	// Specify multiple keys if needed for redundancy rotation.
	// Sort them in the order they should be used for redundancy rotation.
	// +kubebuilder:validation:MinItems=1
	// +required
	PreSharedKeys []PreSharedKey `json:"preSharedKeys"`

	// Policy defines the MACSec policy configuration.
	// +required
	Policy *MacSecPolicy `json:"policy"`
}

// CryptographicAlgorithm is the MKA key-derivation algorithm used for a pre-shared key.
// +kubebuilder:validation:Enum=aes-128-cmac;aes-256-cmac
type CryptographicAlgorithm string

const (
	// AES128CMAC is the AES-128-CMAC key-derivation algorithm.
	AES128CMAC CryptographicAlgorithm = "aes-128-cmac"
	// AES256CMAC is the AES-256-CMAC key-derivation algorithm.
	AES256CMAC CryptographicAlgorithm = "aes-256-cmac"
)

// PreSharedKey defines a single MACsec pre-shared key: a reference to the Secret
// holding the raw key material plus the key metadata.
// +kubebuilder:validation:XValidation:rule="self.endTime > self.startTime",message="endTime must be after startTime"
type PreSharedKey struct {
	// KeyRef references the Secret (in the same namespace) holding the raw key material
	// under the data key "key". The Secret must contain only the key material.
	// +required
	KeyRef LocalObjectReference `json:"keyRef"`

	// ConnectivityKeyName is the connectivity association key name (CKN).
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	ConnectivityKeyName string `json:"connectivityKeyName"`

	// Algorithm is the cryptographic algorithm used for this key.
	// +required
	Algorithm CryptographicAlgorithm `json:"algorithm"`

	// StartTime is when this key becomes active.
	// +required
	StartTime metav1.Time `json:"startTime"`

	// EndTime is when this key expires. Must be after startTime.
	// +required
	// Validates if the string is a valid datetime in RFC3339 format.
	EndTime metav1.Time `json:"endTime"`
}

type MacSecPolicy struct {
	CipherSuite string `json:"cipherSuite,omitempty"`

	// Number of octets in an Ethernet frame that are sent in unencrypted plain-text
	// +required
	ConfidentialityOffset ConfidentialityOffset `json:"confidentialityOffset"`

	// Specifies the key server priority used by the MACsec Key Agreement (MKA) protocol to select the key server when
	// MACsec is enabled using static connectivity association key (CAK) security mode.
	KeyServerPriority uint8 `json:"keyServerPriority"`

	// Number of out-of-order packets that can be received before the secure channel is considered to be inoperable.
	// A value of 0 means that frames are accepted only in the correct order.
	RelayProtection uint16 `json:"relayProtection"`
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
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &MacSec{}, &MacSecList{})
		return nil
	})
}
