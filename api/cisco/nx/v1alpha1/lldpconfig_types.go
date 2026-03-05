// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=lldpconfigs,verbs=get;list;watch

// LLDPConfig defines the Cisco-specific configuration of an LLDP object.
type LLDPConfigSpec struct {
	// InitDelay defines the delay in seconds before LLDP starts sending packets after interface comes up.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=2
	InitDelay int16 `json:"initDelay,omitempty"`

	// HoldTime defines the time in seconds that the receiving device should hold the LLDP information before discarding it.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	// +kubebuilder:default=120
	HoldTime int16 `json:"holdTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=lldpconfigs
// +kubebuilder:resource:singular=lldpconfig

// LLDPConfig is the Schema for the LLDPConfig API
type LLDPConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of LLDP
	// +required
	Spec LLDPConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// LLDPConfigList contains a list of LLDPConfigs
type LLDPConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LLDPConfig `json:"items"`
}

// init registers the LLDPConfig type with the scheme and sets
// itself as a dependency for the LLDP core type.
func init() {
	v1alpha1.RegisterLLDPDependency(GroupVersion.WithKind("LLDPConfig"))
	SchemeBuilder.Register(&LLDPConfig{}, &LLDPConfigList{})
}
