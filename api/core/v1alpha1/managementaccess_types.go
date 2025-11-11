// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagementAccessSpec defines the desired state of ManagementAccess
type ManagementAccessSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Configuration for the gRPC server on the device.
	// Currently, only a single "default" gRPC server is supported.
	// +optional
	// +kubebuilder:default={enabled:true, port:9339}
	GRPC GRPC `json:"grpc,omitzero"`

	// Configuration for the SSH server on the device.
	// +optional
	// +kubebuilder:default={enabled:true, timeout:"10m", sessionLimit:32}
	SSH SSH `json:"ssh,omitzero"`
}

type GRPC struct {
	// Enable or disable the gRPC server on the device.
	// If not specified, the gRPC server is enabled by default.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// The TCP port on which the gRPC server should listen.
	// The range of port-id is from 1024 to 65535.
	// Port 9339 is the default.
	// +optional
	// +kubebuilder:default=9339
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=false
	Port int32 `json:"port"`

	// Name of the certificate that is associated with the gRPC service.
	// The certificate is provisioned through other interfaces on the device,
	// such as e.g. the gNOI certificate management service.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	CertificateID string `json:"certificateId,omitempty"`

	// Enable the gRPC agent to accept incoming (dial-in) RPC requests from a given vrf.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	VrfName string `json:"vrfName"`

	// Additional gNMI configuration for the gRPC server.
	// This may not be supported by all devices.
	// +optional
	// +kubebuilder:default={maxConcurrentCall:8, keepAliveTimeout:"10m"}
	GNMI GNMI `json:"gnmi,omitzero"`
}

type GNMI struct {
	// The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.
	// Configure a limit from 1 through 16. The default limit is 8.
	// +optional
	// +kubebuilder:default=8
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16
	// +kubebuilder:validation:ExclusiveMaximum=false
	MaxConcurrentCall int8 `json:"maxConcurrentCall"`

	// Configure the keepalive timeout for inactive or unauthorized connections.
	// The gRPC agent is expected to periodically send an empty response to the client, on which the client is expected to respond with an empty request.
	// If the client does not respond within the keepalive timeout, the gRPC agent should close the connection.
	// The default interval value is 10 minutes.
	// +optional
	// +kubebuilder:default="10m"
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	KeepAliveTimeout metav1.Duration `json:"keepAliveTimeout"`
}

type SSH struct {
	// Enable or disable the SSH server on the device.
	// If not specified, the SSH server is enabled by default.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// The timeout duration for SSH sessions.
	// If not specified, the default timeout is 10 minutes.
	// +optional
	// +kubebuilder:default="10m"
	// +kubebuilder:validation:Type=string
	Timeout metav1.Duration `json:"timeout,omitzero"`

	// The maximum number of concurrent SSH sessions allowed.
	// If not specified, the default limit is 32.
	// +optional
	// +kubebuilder:default=32
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	// +kubebuilder:validation:ExclusiveMaximum=false
	SessionLimit int8 `json:"sessionLimit,omitempty"`
}

// ManagementAccessStatus defines the observed state of ManagementAccess.
type ManagementAccessStatus struct {
	// The conditions are a list of status objects that describe the state of the ManagementAccess.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=managementaccesses
// +kubebuilder:resource:singular=managementaccess
// +kubebuilder:resource:shortName=mgmt;mgmtaccess
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ManagementAccess is the Schema for the managementaccesses API
type ManagementAccess struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ManagementAccessSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ManagementAccessStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (ma *ManagementAccess) GetConditions() []metav1.Condition {
	return ma.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (ma *ManagementAccess) SetConditions(conditions []metav1.Condition) {
	ma.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ManagementAccessList contains a list of ManagementAccess
type ManagementAccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagementAccess `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagementAccess{}, &ManagementAccessList{})
}
