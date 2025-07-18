// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	// Parameters is a raw block of JSON that contains the provider-specific
	// configuration. The schema of this block is not validated by Kubernetes
	// and remains responsibility of the provider implementation.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +required
	Parameters runtime.RawExtension `json:"parameters"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=providerconfigs
// +kubebuilder:resource:singular=providerconfig
// +kubebuilder:resource:shortName=config

// ProviderConfig is the Schema for the providerconfigs API.
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec ProviderConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
