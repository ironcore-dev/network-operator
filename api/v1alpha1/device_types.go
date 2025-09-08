// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceSpec defines the desired state of Device.
type DeviceSpec struct {
	// Endpoint contains the connection information for the device.
	// +required
	Endpoint *Endpoint `json:"endpoint"`

	// Bootstrap is an optional configuration for the device bootstrap process.
	// It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.
	// +optional
	Bootstrap *Bootstrap `json:"bootstrap,omitempty"`

	// Configuration for the gRPC server on the device.
	// Currently, only a single "default" gRPC server is supported.
	// +optional
	GRPC *GRPC `json:"grpc,omitempty"`
}

type Endpoint struct {
	// Address is the management address of the device provided as <ip:port>.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}:\d{1,5}$`
	// +required
	Address string `json:"address"`

	// SecretRef is name of the authentication secret for the device containing the username and password.
	// The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: 'username' and 'password'.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`

	// Transport credentials for grpc connection to the switch.
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

type TLS struct {
	// The CA certificate to verify the server's identity.
	// +required
	CA *corev1.SecretKeySelector `json:"ca"`

	// The client certificate and private key to use for mutual TLS authentication.
	// Leave empty if mTLS is not desired.
	// +optional
	Certificate *CertificateSource `json:"certificate,omitempty"`
}

// Bootstrap defines the configuration for device bootstrap.
type Bootstrap struct {
	// Template defines the multiline string template that contains the initial configuration for the device.
	// +required
	Template *TemplateSource `json:"template"`
}

type GRPC struct {
	// The TCP port on which the gRPC server should listen.
	// The range of port-id is from 1024 to 65535.
	// Port 9339 is the default.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=9339
	// +optional
	Port int32 `json:"port"`

	// Name of the certificate that is associated with the gRPC service.
	// The certificate is provisioned through other interfaces on the device,
	// such as e.g. the gNOI certificate management service.
	// +optional
	CertificateID string `json:"certificateId,omitempty"`

	// Enable the gRPC agent to accept incoming (dial-in) RPC requests from a given network instance.
	// +optional
	NetworkInstance string `json:"networkInstance,omitempty"`

	// Additional gNMI configuration for the gRPC server.
	// This may not be supported by all devices.
	// +optional
	GNMI *GNMI `json:"gnmi,omitempty"`
}

type GNMI struct {
	// The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.
	// Configure a limit from 1 through 16. The default limit is 8.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=8
	// +optional
	MaxConcurrentCall int8 `json:"maxConcurrentCall"`

	// Configure the keepalive timeout for inactive or unauthorized connections.
	// The gRPC agent is expected to periodically send an empty response to the client, on which the client is expected to respond with an empty request.
	// If the client does not respond within the keepalive timeout, the gRPC agent should close the connection.
	// The default interval value is 10 minutes.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +kubebuilder:default="10m"
	// +optional
	KeepAliveTimeout metav1.Duration `json:"keepAliveTimeout"`

	// Configure the minimum sample interval for the gNMI telemetry stream.
	// Once per stream sample interval, the switch sends the current values for all specified paths.
	// The default value is 10 seconds.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +kubebuilder:default="10s"
	// +optional
	MinSampleInterval metav1.Duration `json:"minSampleInterval"`
}

// TemplateSource defines a source for template content.
// It can be provided inline, or as a reference to a Secret or ConfigMap.
//
// +kubebuilder:validation:XValidation:rule="[has(self.inline), has(self.secretRef), has(self.configMapRef)].filter(x, x).size() == 1",message="exactly one of 'inline', 'secretRef', or 'configMapRef' must be specified"
type TemplateSource struct {
	// Inline template content
	// +optional
	Inline *string `json:"inline,omitempty"`

	// Reference to a Secret containing the template
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`

	// Reference to a ConfigMap containing the template
	// +optional
	ConfigMapRef *corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
}

// CertificateSource represents a source for the value of a certificate.
type CertificateSource struct {
	// Secret containing the certificate.
	// The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'.
	// +required
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// DeviceStatus defines the observed state of Device.
type DeviceStatus struct {
	// Phase represents the current phase of the Device.
	// +kubebuilder:default=Pending
	// +required
	Phase DevicePhase `json:"phase,omitempty"`

	// The conditions are a list of status objects that describe the state of the Device.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// DevicePhase represents the current phase of the Device as it's being provisioned and managed by the operator.
//
// +kubebuilder:validation:Enum=Pending;Provisioning;Active;Failed
type DevicePhase string

const (
	// DevicePhasePending indicates that the device is pending and has not yet been provisioned.
	DevicePhasePending DevicePhase = "Pending"
	// DevicePhaseProvisioning indicates that the device is being provisioned.
	DevicePhaseProvisioning DevicePhase = "Provisioning"
	// DevicePhaseActive indicates that the device has been successfully provisioned and is now ready for use.
	DevicePhaseActive DevicePhase = "Active"
	// DevicePhaseFailed indicates that the device provisioning has failed.
	DevicePhaseFailed DevicePhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=devices
// +kubebuilder:resource:singular=device
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint.address`
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Device is the Schema for the devices API.
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec DeviceSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status DeviceStatus `json:"status,omitempty"`
}

// GetSecretRefs returns the list of secrets referenced in the [Device] resource.
func (d *Device) GetSecretRefs() []corev1.SecretReference {
	refs := []corev1.SecretReference{}
	if d.Spec.Endpoint.SecretRef != nil {
		refs = append(refs, *d.Spec.Endpoint.SecretRef)
	}
	if d.Spec.Endpoint.TLS != nil {
		refs = append(refs, corev1.SecretReference{Name: d.Spec.Endpoint.TLS.CA.Name})
		if d.Spec.Endpoint.TLS.Certificate != nil {
			refs = append(refs, *d.Spec.Endpoint.TLS.Certificate.SecretRef)
		}
	}
	if d.Spec.Bootstrap != nil && d.Spec.Bootstrap.Template != nil {
		if d.Spec.Bootstrap.Template.SecretRef != nil {
			refs = append(refs, corev1.SecretReference{Name: d.Spec.Bootstrap.Template.SecretRef.Name})
		}
	}
	for i := range refs {
		if refs[i].Namespace == "" {
			refs[i].Namespace = d.Namespace
		}
	}
	return refs
}

// GetConfigMapRefs returns the list of configmaps referenced in the [Device] resource.
func (d *Device) GetConfigMapRefs() []corev1.ObjectReference {
	refs := []corev1.ObjectReference{}
	if d.Spec.Bootstrap != nil && d.Spec.Bootstrap.Template != nil {
		if d.Spec.Bootstrap.Template.ConfigMapRef != nil {
			refs = append(refs, corev1.ObjectReference{Name: d.Spec.Bootstrap.Template.ConfigMapRef.Name})
		}
	}
	for i := range refs {
		if refs[i].Namespace == "" {
			refs[i].Namespace = d.Namespace
		}
	}
	return refs
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device.
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
