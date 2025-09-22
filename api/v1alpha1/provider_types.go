// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderConfigReference is a reference to a resource holding the provider-specific configuration of an object.
type ProviderConfigReference struct {
	// Kind of the resource being referenced.
	// Kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`
	Kind string `json:"kind"`

	// Name of the resource being referenced.
	// Name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`

	// APIVersion is the api group version of the resource being referenced.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	//+kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?([a-z0-9]([-a-z0-9]*[a-z0-9])?)$`
	APIVersion string `json:"apiVersion"`
}

const (
	SonicProviderType     = "SonicProvider"
	OpenconfigProviderType = "OpenconfigProvider"
)

type SonicProviderConfigSpec struct {
	Address  string `json:"address"`
	Port    int32  `json:"port,omitempty"`
}

type SonicProviderConfigStatus struct {
	
}

type SonicProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SonicProviderConfigSpec   `json:"spec,omitempty"`
	Status SonicProviderConfigStatus `json:"status,omitempty"`
}

type SonicProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SonicProviderConfig `json:"items"`
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

// CertificateSource represents a source for the value of a certificate.
type CertificateSource struct {
	// Secret containing the certificate.
	// The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'.
	// +required
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// PasswordSource represents a source for the value of a password.
type PasswordSource struct {
	// Selects a key of a secret.
	// +required
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
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

type OpenconfigProviderConfigSpec struct {
	// Endpoint contains the connection information for the device.
	// +required
	Endpoint *Endpoint `json:"endpoint"`
}

type OpenconfigProviderConfigStatus struct {

}

type OpenconfigProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenconfigProviderConfigSpec   `json:"spec,omitempty"`
	Status OpenconfigProviderConfigStatus `json:"status,omitempty"`
}

type OpenconfigProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenconfigProviderConfig `json:"items"`
}

