// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=interfaceconfigs,verbs=get;list;watch

// InterfaceConfigSpec defines the desired state of InterfaceConfig
type InterfaceConfigSpec struct {
	// SpanningTree defines the spanning tree configuration for the interface.
	// +optional
	SpanningTree *SpanningTree `json:"spanningTree,omitempty"`

	// BufferBoost defines the buffer boost configuration for the interface.
	// Buffer boost increases the shared buffer space allocation for the interface.
	// +optional
	BufferBoost *BufferBoost `json:"bufferBoost,omitempty"`
}

// SpanningTree defines the spanning tree configuration for an interface.
type SpanningTree struct {
	// PortType defines the spanning tree port type.
	// +required
	PortType SpanningTreePortType `json:"portType"`

	// BPDUGuard enables BPDU guard on the interface.
	// When enabled, the port is shut down if a BPDU is received.
	// +optional
	BPDUGuard *bool `json:"bpduGuard,omitempty"`

	// BPDUFilter enables BPDU filter on the interface.
	// When enabled, BPDUs are not sent or received on the port.
	// +optional
	BPDUFilter *bool `json:"bpduFilter,omitempty"`
}

// BufferBoost defines the buffer boost configuration for an interface.
type BufferBoost struct {
	// Enabled indicates whether buffer boost is enabled on the interface.
	// Maps to CLI command: hardware profile buffer boost
	// +required
	Enabled bool `json:"enabled"`
}

// SpanningTreePortType represents the spanning tree port type.
// +kubebuilder:validation:Enum=Normal;Edge;Network
type SpanningTreePortType string

const (
	// SpanningTreePortTypeNormal indicates a normal spanning tree port.
	SpanningTreePortTypeNormal SpanningTreePortType = "Normal"
	// SpanningTreePortTypeEdge indicates an edge port (connects to end devices).
	SpanningTreePortTypeEdge SpanningTreePortType = "Edge"
	// SpanningTreePortTypeNetwork indicates a network port (connects to other switches).
	SpanningTreePortTypeNetwork SpanningTreePortType = "Network"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=interfaceconfigs
// +kubebuilder:resource:singular=interfaceconfigs
// +kubebuilder:resource:shortName=intcfg

// InterfaceConfig is the Schema for the interfaceconfigs API
type InterfaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec InterfaceConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// InterfaceConfigList contains a list of InterfaceConfig
type InterfaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InterfaceConfig `json:"items"`
}

func init() {
	v1alpha1.RegisterInterfaceDependency(GroupVersion.WithKind("InterfaceConfig"))
	SchemeBuilder.Register(&InterfaceConfig{}, &InterfaceConfigList{})
}
