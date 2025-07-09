// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceSpec defines the desired state of Device.
type DeviceSpec struct {
	// Endpoint is the management address of the device provided as <ip:port>.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}:\d{1,5}$`
	// +required
	Endpoint string `json:"endpoint"`

	// SecretRef is name of the authentication secret for the device containing the username and password.
	// The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: 'username' and 'password'.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`

	// Bootstrap is an optional configuration for the device bootstrap process.
	// It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.
	// +optional
	Bootstrap *Bootstrap `json:"bootstrap,omitempty"`
}

// Bootstrap defines the configuration for device bootstrap.
type Bootstrap struct {
	// Template defines the multiline string template that contains the initial configuration for the device.
	// +required
	Template *TemplateSource `json:"template"`
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
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"

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
