// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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

	// LACP defines LACP options for PortChannel (Aggregate) interfaces.
	// +optional
	LACP *InterfaceConfigLACP `json:"lacp,omitempty"`

	// EVPNMultihoming defines EVPN ESI multihoming settings for the interface.
	// +optional
	EVPNMultihoming *EVPNMultihoming `json:"evpnMultihoming,omitempty"`

	// IPv6 defines IPv6 settings for the interface that have no equivalent in
	// the core Interface API. Neighbor Discovery settings are only managed while
	// this is set: other ND settings on the device are left untouched, and
	// removing it leaves the last applied settings on the device.
	// +optional
	IPv6 *InterfaceConfigIPv6 `json:"ipv6,omitempty"`
}

// InterfaceConfigIPv6 defines IPv6 settings for an interface.
type InterfaceConfigIPv6 struct {
	// SuppressRouterAdvertisement stops the interface from sending IPv6 Router
	// Advertisements. NX-OS sends them by default. Neighbours that discover
	// each other over their link-local addresses, such as unnumbered BGP peers,
	// depend on them, so this must stay disabled for such interfaces.
	// Only applied to interfaces that carry IPv6 configuration.
	// Maps to CLI command: ipv6 nd suppress-ra
	// +required
	SuppressRouterAdvertisement bool `json:"suppressRouterAdvertisement"`

	// RouterAdvertisementInterval is the maximum interval between periodic
	// IPv6 Router Advertisements, between 4s and 30m in whole seconds. The
	// minimum interval is derived from it the way NX-OS does for the CLI
	// command: a third of this value, but no less than 3s.
	// Unnumbered BGP peers only discover each other once an advertisement is
	// received, so a short interval speeds up session establishment.
	// If not specified, the NX-OS default of 600s applies.
	// Maps to CLI command: ipv6 nd ra-interval
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	RouterAdvertisementInterval *metav1.Duration `json:"routerAdvertisementInterval,omitempty"`
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

// EVPNMultihoming defines EVPN ESI multihoming settings for an interface.
type EVPNMultihoming struct {
	// CoreTracking enables core-link tracking on the interface.
	// When enabled on uplink (core) interfaces, the switch shuts down
	// ESI-attached access links if all tracked core-links go down,
	// preventing traffic blackholing.
	// +required
	CoreTracking bool `json:"coreTracking"`
}

// InterfaceConfigLACP defines LACP options for PortChannel interfaces.
type InterfaceConfigLACP struct {
	// VPCConvergence enables faster LACP convergence in a vPC topology.
	// +optional
	VPCConvergence *bool `json:"vpcConvergence,omitempty"`

	// SuspendIndividual controls whether a member port is suspended when
	// LACP PDUs are not received. Set to false to keep the port forwarding.
	// +optional
	SuspendIndividual *bool `json:"suspendIndividual,omitempty"`
}

// SpanningTreePortType represents the spanning tree port type.
// +kubebuilder:validation:Enum=Normal;Edge;Network;Trunk
type SpanningTreePortType string

const (
	// SpanningTreePortTypeNormal indicates a normal spanning tree port.
	SpanningTreePortTypeNormal SpanningTreePortType = "Normal"
	// SpanningTreePortTypeEdge indicates an edge port (connects to end devices).
	SpanningTreePortTypeEdge SpanningTreePortType = "Edge"
	// SpanningTreePortTypeTrunk indicates a trunk port performing spanning tree calculations for multiple VLANs (connects to end devices and carries multiple VLANs).
	SpanningTreePortTypeTrunk SpanningTreePortType = "Trunk"
	// SpanningTreePortTypeNetwork indicates a network port (connects to other switches).
	SpanningTreePortTypeNetwork SpanningTreePortType = "Network"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=interfaceconfigs
// +kubebuilder:resource:singular=interfaceconfig
// +kubebuilder:resource:shortName=nxint

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
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &InterfaceConfig{}, &InterfaceConfigList{})
		return nil
	})
}
