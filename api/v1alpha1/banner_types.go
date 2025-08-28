// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BannerSpec defines the desired state of Banner
type BannerSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	DeviceName string `json:"deviceName,omitempty"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the interface to its provider-specific configuration.
	// Immutable.
	// +optional
	ProviderConfigRef *ProviderConfigReference `json:"providerConfigRef,omitempty"`

	// MOTD banner to display on login.
	// +required
	Message *TemplateSource `json:"message,omitempty"`
}

// BannerStatus defines the observed state of Banner.
type BannerStatus struct {
	// The conditions are a list of status objects that describe the state of the Banner.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=banners
// +kubebuilder:resource:singular=banner
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Banner is the Schema for the banners API
type Banner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec BannerSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status BannerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BannerList contains a list of Banner
type BannerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Banner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Banner{}, &BannerList{})
}
