// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceSpec defines the desired state of Device.
type DeviceSpec struct {
	// Endpoint contains the connection information for the device.
	// +required
	Endpoint Endpoint `json:"endpoint"`

	// Bootstrap is an optional configuration for the device bootstrap process.
	// It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.
	// +optional
	Bootstrap *Bootstrap `json:"bootstrap,omitempty"`
}

// Endpoint contains the connection information for the device.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.secretRef) || has(self.secretRef)", message="SecretRef is required once set"
type Endpoint struct {
	// Address is the management address of the device provided as <ip:port>.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}:\d{1,5}$`
	// +required
	Address string `json:"address"`

	// SecretRef is name of the authentication secret for the device containing the username and password.
	// The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: 'username' and 'password'.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// Transport credentials for grpc connection to the switch.
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

type TLS struct {
	// The CA certificate to verify the server's identity.
	// +required
	CA SecretKeySelector `json:"ca"`

	// The client certificate and private key to use for mutual TLS authentication.
	// Leave empty if mTLS is not desired.
	// +optional
	Certificate *CertificateSource `json:"certificate,omitempty"`
}

// Bootstrap defines the configuration for device bootstrap.
type Bootstrap struct {
	// Template defines the multiline string template that contains the initial configuration for the device.
	// +required
	Template TemplateSource `json:"template"`
}

// TemplateSource defines a source for template content.
// It can be provided inline, or as a reference to a Secret or ConfigMap.
//
// +kubebuilder:validation:XValidation:rule="[has(self.inline), has(self.secretRef), has(self.configMapRef)].filter(x, x).size() == 1",message="exactly one of 'inline', 'secretRef', or 'configMapRef' must be specified"
type TemplateSource struct {
	// Inline template content
	// +optional
	// +kubebuilder:validation:MinLength=1
	Inline *string `json:"inline,omitempty"`

	// Reference to a Secret containing the template
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`

	// Reference to a ConfigMap containing the template
	// +optional
	ConfigMapRef *ConfigMapKeySelector `json:"configMapRef,omitempty"`
}

// CertificateSource represents a source for the value of a certificate.
type CertificateSource struct {
	// Secret containing the certificate.
	// The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'.
	// +required
	SecretRef SecretReference `json:"secretRef,omitempty"`
}

// DeviceStatus defines the observed state of Device.
type DeviceStatus struct {
	// Phase represents the current phase of the Device.
	// +kubebuilder:default=Pending
	// +required
	Phase DevicePhase `json:"phase,omitempty"`

	// Manufacturer is the manufacturer of the Device.
	// +optional
	Manufacturer string `json:"manufacturer,omitempty"`

	// Model is the model identifier of the Device.
	// +optional
	Model string `json:"model,omitempty"`

	// SerialNumber is the serial number of the Device.
	// +optional
	SerialNumber string `json:"serialNumber,omitempty"`

	// FirmwareVersion is the firmware version running on the Device.
	// +optional
	FirmwareVersion string `json:"firmwareVersion,omitempty"`

	// Ports is the list of ports on the Device.
	// +optional
	Ports []DevicePort `json:"ports,omitempty"`

	// PostSummary shows a summary of the port configured, grouped by type, e.g. "1/4 (10g), 3/64 (100g)".
	// +optional
	PostSummary string `json:"portSummary,omitempty"`

	// The conditions are a list of status objects that describe the state of the Device.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type DevicePort struct {
	// Name is the name of the port.
	// +required
	Name string `json:"name"`

	// Type is the type of the port, e.g. "10g".
	// +optional
	Type string `json:"type,omitempty"`

	// SupportedSpeedsGbps is the list of supported speeds in Gbps for this port.
	// +optional
	SupportedSpeedsGbps []int32 `json:"supportedSpeedsGbps,omitempty"`

	// Transceiver is the type of transceiver plugged into the port, if any.
	// +optional
	Trasceiver string `json:"transceiver,omitempty"`

	// InterfaceRef is the reference to the corresponding Interface resource
	// configuring this port, if any.
	// +optional
	InterfaceRef *LocalObjectReference `json:"interfaceName,omitzero"`
}

// DevicePhase represents the current phase of the Device as it's being provisioned and managed by the operator.
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
// +kubebuilder:printcolumn:name="Manufacturer",type=string,JSONPath=".status.manufacturer",priority=1
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=".status.model",priority=1
// +kubebuilder:printcolumn:name="SerialNumber",type=string,JSONPath=".status.serialNumber",priority=1
// +kubebuilder:printcolumn:name="FirmwareVersion",type=string,JSONPath=".status.firmwareVersion",priority=1
// +kubebuilder:printcolumn:name="Ports",type=string,JSONPath=".status.portSummary",priority=1
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
func (d *Device) GetSecretRefs() []SecretReference {
	refs := []SecretReference{}
	if d.Spec.Endpoint.SecretRef != nil {
		refs = append(refs, *d.Spec.Endpoint.SecretRef)
	}
	if d.Spec.Endpoint.TLS != nil {
		refs = append(refs, d.Spec.Endpoint.TLS.CA.SecretReference)
		if d.Spec.Endpoint.TLS.Certificate != nil {
			refs = append(refs, d.Spec.Endpoint.TLS.Certificate.SecretRef)
		}
	}
	if d.Spec.Bootstrap != nil {
		if d.Spec.Bootstrap.Template.SecretRef != nil {
			refs = append(refs, d.Spec.Bootstrap.Template.SecretRef.SecretReference)
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
func (d *Device) GetConfigMapRefs() []ConfigMapReference {
	refs := []ConfigMapReference{}
	if d.Spec.Bootstrap != nil {
		if d.Spec.Bootstrap.Template.ConfigMapRef != nil {
			refs = append(refs, d.Spec.Bootstrap.Template.ConfigMapRef.ConfigMapReference)
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
