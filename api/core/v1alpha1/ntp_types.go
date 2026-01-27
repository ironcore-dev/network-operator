// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NTPSpec defines the desired state of NTP
type NTPSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the NTP to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether NTP is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// Source interface for all NTP traffic.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	SourceInterfaceName string `json:"sourceInterfaceName"`

	// NTP servers.
	// +required
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MinItems=1
	Servers []NTPServer `json:"servers"`
}

type NTPServer struct {
	// Hostname/IP address of the NTP server.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// Indicates whether this server should be preferred or not.
	// +optional
	// +kubebuilder:default=false
	Prefer bool `json:"prefer,omitempty"`

	// The name of the vrf used to communicate with the NTP server.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	VrfName string `json:"vrfName,omitempty"`
}

// NTPStatus defines the observed state of NTP.
type NTPStatus struct {
	// The conditions are a list of status objects that describe the state of the NTP.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ntp
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NTP is the Schema for the ntp API
type NTP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec NTPSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status NTPStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (ntp *NTP) GetConditions() []metav1.Condition {
	return ntp.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (ntp *NTP) SetConditions(conditions []metav1.Condition) {
	ntp.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// NTPList contains a list of NTP
type NTPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NTP `json:"items"`
}

var (
	NTPDependencies   []schema.GroupVersionKind
	ntpDependenciesMu sync.Mutex
)

func RegisterNTPDependency(gvk schema.GroupVersionKind) {
	ntpDependenciesMu.Lock()
	defer ntpDependenciesMu.Unlock()
	NTPDependencies = append(NTPDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&NTP{}, &NTPList{})
}
