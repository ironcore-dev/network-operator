// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CertificateSpec defines the desired state of Certificate
type CertificateSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Certificate to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// The certificate management id.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ID is immutable"
	ID string `json:"id"`

	// Secret containing the certificate source.
	// The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'.
	// +required
	SecretRef SecretReference `json:"secretRef"`
}

// CertificateStatus defines the observed state of Certificate.
type CertificateStatus struct {
	// The conditions are a list of status objects that describe the state of the Certificate.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=certificates
// +kubebuilder:resource:singular=certificate
// +kubebuilder:resource:shortName=netcert
// +kubebuilder:printcolumn:name="Certificate",type=string,JSONPath=`.spec.id`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Certificate is the Schema for the certificates API
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec CertificateSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status CertificateStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (cert *Certificate) GetConditions() []metav1.Condition {
	return cert.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (cert *Certificate) SetConditions(conditions []metav1.Condition) {
	cert.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// CertificateList contains a list of Certificate
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

var (
	CertificateDependencies   []schema.GroupVersionKind
	certificateDependenciesMu sync.Mutex
)

func RegisterCertificateDependency(gvk schema.GroupVersionKind) {
	certificateDependenciesMu.Lock()
	defer certificateDependenciesMu.Unlock()
	CertificateDependencies = append(CertificateDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}
