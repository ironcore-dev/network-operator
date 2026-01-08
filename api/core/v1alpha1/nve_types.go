// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NetworkVirtualizationEdgeSpec defines the desired state of a Network Virtualization Edge (NVE).
// +kubebuilder:validation:XValidation:rule="!has(self.anycastSourceInterfaceRef) || self.anycastSourceInterfaceRef.name != self.sourceInterfaceRef.name",message="anycastSourceInterfaceRef.name must differ from sourceInterfaceRef.name"
type NetworkVirtualizationEdgeSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration for this NVE.
	// If not specified the provider applies the target platform's default settings.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether the interface is administratively up or down.
	// +required
	AdminState AdminState `json:"adminState"`

	// SourceInterface is the reference to the loopback interface used for the primary NVE IP address.
	// +required
	SourceInterfaceRef LocalObjectReference `json:"sourceInterfaceRef"`

	// AnycastSourceInterfaceRef is the reference to the loopback interface used for anycast NVE IP address.
	// +optional
	AnycastSourceInterfaceRef *LocalObjectReference `json:"anycastSourceInterfaceRef,omitempty"`

	// SuppressARP indicates whether ARP suppression is enabled for this NVE.
	// +optional
	// +kubebuilder:default=false
	SuppressARP bool `json:"suppressARP"`

	// HostReachability specifies the method used for host reachability.
	// +required
	HostReachability HostReachabilityType `json:"hostReachability"`

	// MulticastGroups defines multicast group addresses for BUM traffic.
	// +optional
	MulticastGroups *MulticastGroups `json:"multicastGroups,omitzero"`

	// AnycastGateway defines the distributed anycast gateway configuration.
	// This enables multiple NVEs to share the same gateway IP and MAC
	// for active-active first-hop redundancy.
	// +optional
	AnycastGateway *AnycastGateway `json:"anycastGateway,omitzero"`
}

// HostReachabilityType defines the method used for host reachability.
// +kubebuilder:validation:Enum=FloodAndLearn;BGP
type HostReachabilityType string

const (
	// HostReachabilityTypeBGP uses BGP EVPN control-plane for MAC/IP advertisement.
	HostReachabilityTypeBGP HostReachabilityType = "BGP"
	// HostReachabilityTypeFloodAndLearn uses data-plane learning for MAC addresses.
	HostReachabilityTypeFloodAndLearn HostReachabilityType = "FloodAndLearn"
)

// MulticastGroups defines multicast group addresses for overlay BUM traffic.
// Only supports IPv4 multicast addresses.
type MulticastGroups struct {
	// L2 is the multicast group for Layer 2 VNIs (BUM traffic in bridged VLANs).
	// +optional
	// +kubebuilder:validation:Format=ipv4
	L2 string `json:"l2,omitempty"`

	// L3 is the multicast group for Layer 3 VNIs (BUM traffic in routed VRFs).
	// +optional
	// +kubebuilder:validation:Format=ipv4
	L3 string `json:"l3,omitempty"`
}

// AnycastGateway defines distributed anycast gateway configuration.
// Multiple NVEs in the fabric share the same virtual MAC address,
// enabling active-active default gateway redundancy for hosts.
type AnycastGateway struct {
	// VirtualMAC is the shared MAC address used by all NVEs in the fabric
	// for anycast gateway functionality on RoutedVLAN (SVI) interfaces.
	// All switches in the fabric must use the same MAC address.
	// Format: IEEE 802 MAC-48 address (e.g., "00:00:5E:00:01:01")
	// +required
	// +kubebuilder:validation:Pattern=`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`
	VirtualMAC string `json:"virtualMAC"`
}

// NetworkVirtualizationEdgeStatus defines the observed state of the NVE.
type NetworkVirtualizationEdgeStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the NVE resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The conditions are a list of status objects that describe the state of the NVE.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// SourceInterfaceName is the resolved source interface IP address used for NVE encapsulation.
	SourceInterfaceName string `json:"sourceInterfaceName,omitempty"`

	// AnycastSourceInterfaceName is the resolved anycast source interface IP address used for NVE encapsulation.
	AnycastSourceInterfaceName string `json:"anycastSourceInterfaceName,omitempty"`

	// HostReachability indicates the actual method used for host reachability.
	HostReachability string `json:"hostReachability,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=networkvirtualizationedges
// +kubebuilder:resource:singular=networkvirtualizationedge
// +kubebuilder:resource:shortName=nve
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="SrcIf",type=string,JSONPath=`.status.sourceInterfaceName`
// +kubebuilder:printcolumn:name="AnycastSrcIf",type=string,JSONPath=`.status.anycastSourceInterfaceName`
// +kubebuilder:printcolumn:name="HostReachability",type=string,JSONPath=`.status.hostReachability`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkVirtualizationEdge is the Schema for the networkvirtualizationedges API
// The NVE resource is the equivalent to an Endpoint for a Network Virtualization Overlay Object in OpenConfig (`nvo:Ep`).
type NetworkVirtualizationEdge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec NetworkVirtualizationEdgeSpec `json:"spec"`

	// +optional
	Status NetworkVirtualizationEdgeStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (in *NetworkVirtualizationEdge) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (in *NetworkVirtualizationEdge) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// NetworkVirtualizationEdgeList contains a list of NetworkVirtualizationEdges
type NetworkVirtualizationEdgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkVirtualizationEdge `json:"items"`
}

var (
	NetworkVirtualizationEdgeDependencies   []schema.GroupVersionKind
	networkVirtualizationEdgeDependenciesMu sync.Mutex
)

// RegisterNetworkVirtualizationEdgeDependency adds GVKs to the NVE dependency registry.This function is typically
// called during package initialization by provider implementations (e.g., NVOConfig from cisco/nx/v1alpha1)
// to declare themselves as valid ProviderConfigRef targets.
func RegisterNetworkVirtualizationEdgeDependency(gvk schema.GroupVersionKind) {
	networkVirtualizationEdgeDependenciesMu.Lock()
	defer networkVirtualizationEdgeDependenciesMu.Unlock()
	NetworkVirtualizationEdgeDependencies = append(NetworkVirtualizationEdgeDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&NetworkVirtualizationEdge{}, &NetworkVirtualizationEdgeList{})
}
