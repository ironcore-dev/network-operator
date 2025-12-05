// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=portchannelconfigs,verbs=get;list;watch

// PortChannelConfig defines the desired state of a PortChannel.
// Use as provider specific resource for core resources of type `Interface` (sub-type `Aggregate`).
type PortChannelConfigSpec struct {
	// VPCPeerLink indicates whether this PortChannel is part of the vPC peer link.
	// +optional
	// +kubebuilder:default=false
	VPCPeerLink bool `json:"vpcPeerLink,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=portchannelconfigs
// +kubebuilder:resource:singular=portchannelconfig
// +kubebuilder:resource:shortName=pccfg

// PortChannelConfig is the Schema for the NVE API
type PortChannelConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NVE
	// +required
	Spec PortChannelConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// NVEList contains a list of NVE
type PortChannelConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PortChannelConfig `json:"items"`
}

// init registers the PortChannelConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the NVE core type.
func init() {
	v1alpha1.RegisterInterfaceDependency(v1alpha1.InterfaceTypeAggregate, GroupVersion.WithKind("PortChannelConfig"))
	SchemeBuilder.Register(&PortChannelConfig{}, &PortChannelConfigList{})
}
