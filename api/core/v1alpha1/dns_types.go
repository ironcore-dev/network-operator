// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DNSSpec defines the desired state of DNS
type DNSSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the DNS to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether DNS is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// Default domain name that the device uses to complete unqualified hostnames.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Format=hostname
	Domain string `json:"domain"`

	// A list of DNS servers to use for address resolution.
	// +optional
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=6
	Servers []NameServer `json:"servers,omitempty"`

	// Source interface for all DNS traffic.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	SourceInterfaceName string `json:"sourceInterfaceName,omitempty"`
}

type NameServer struct {
	// The Hostname or IP address of the DNS server.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// The name of the vrf used to communicate with the DNS server.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	VrfName string `json:"vrfName,omitempty"`
}

// DNSStatus defines the observed state of DNS.
type DNSStatus struct {
	// The conditions are a list of status objects that describe the state of the DNS.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dns
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domain`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DNS is the Schema for the dns API
type DNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec DNSSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status DNSStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (dns *DNS) GetConditions() []metav1.Condition {
	return dns.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (dns *DNS) SetConditions(conditions []metav1.Condition) {
	dns.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// DNSList contains a list of DNS
type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNS `json:"items"`
}

var (
	DNSDependencies   []schema.GroupVersionKind
	dnsDependenciesMu sync.Mutex
)

func RegisterDNSDependency(gvk schema.GroupVersionKind) {
	dnsDependenciesMu.Lock()
	defer dnsDependenciesMu.Unlock()
	DNSDependencies = append(DNSDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&DNS{}, &DNSList{})
}
