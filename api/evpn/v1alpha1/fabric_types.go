// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// FabricSpec defines the desired state of Fabric.
type FabricSpec struct {
	// deviceSelector identifies which devices are members of this fabric.
	// All devices whose labels match this selector will be enrolled.
	// A device must not match the deviceSelector of more than one Fabric;
	// overlapping selectors lead to undefined behaviour as both controllers
	// will compete for ownership of the device's sub-resources.
	// +required
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`

	// loopbacks configures IP address allocation for loopback interfaces on
	// fabric devices.
	// +required
	Loopbacks FabricLoopbacksSpec `json:"loopbacks"`

	// underlay defines the underlay routing configuration for the fabric.
	// +required
	Underlay FabricUnderlaySpec `json:"underlay"`

	// overlay defines the overlay control-plane configuration for the fabric.
	// +required
	Overlay FabricOverlaySpec `json:"overlay"`

	// bum defines how BUM traffic is forwarded across the fabric.
	// +required
	BUM FabricBUMSpec `json:"bum"`

	// vtep identifies the VTEP devices and configures their anycast gateway.
	// +required
	VTEP FabricVTEPSpec `json:"vtep"`
}

// FabricLoopbacksSpec configures IP address allocation for loopback interfaces.
type FabricLoopbacksSpec struct {
	// ipAddressPoolRef references the IPAddressPool from which loopback addresses
	// are allocated for devices in the fabric.
	// +required
	IPAddressPoolRef corev1alpha1.LocalObjectReference `json:"ipAddressPoolRef"`
}

// FabricUnderlaySpec defines the underlay network configuration.
type FabricUnderlaySpec struct {
	// protocol is the routing protocol used to build IP reachability across the
	// fabric underlay.
	// +required
	Protocol UnderlayProtocol `json:"protocol"`

	// interfaceSelector identifies which interfaces participate in the underlay.
	// Interfaces on fabric devices matching these labels will be enrolled in the
	// underlay routing process.
	// +required
	InterfaceSelector metav1.LabelSelector `json:"interfaceSelector"`

	// addressing configures how IP addresses are assigned to underlay interfaces.
	// +required
	Addressing FabricUnderlayAddressingSpec `json:"addressing"`
}

// UnderlayProtocol is the routing protocol used for the underlay network.
// +kubebuilder:validation:Enum=OSPF;ISIS
type UnderlayProtocol string

const (
	// UnderlayProtocolOSPF uses OSPF for underlay routing.
	UnderlayProtocolOSPF UnderlayProtocol = "OSPF"
	// UnderlayProtocolISIS uses IS-IS for underlay routing.
	UnderlayProtocolISIS UnderlayProtocol = "ISIS"
)

// FabricUnderlayAddressingSpec configures how IP addresses are assigned to
// underlay point-to-point links.
// +kubebuilder:validation:XValidation:rule="has(self.ipPrefixPoolRef) != self.unnumbered",message="exactly one of ipPrefixPoolRef or unnumbered must be set"
type FabricUnderlayAddressingSpec struct {
	// ipPrefixPoolRef references the IPPrefixPool from which point-to-point
	// prefixes are allocated for underlay interfaces.
	// +optional
	IPPrefixPoolRef *corev1alpha1.LocalObjectReference `json:"ipPrefixPoolRef,omitempty"`

	// unnumbered controls whether underlay interfaces use unnumbered addressing
	// (borrowing from loopback0) instead of dedicated point-to-point addresses.
	// +optional
	// +kubebuilder:default=false
	Unnumbered bool `json:"unnumbered,omitempty"`
}

// OverlayProtocol is the control-plane protocol used for the overlay network.
// +kubebuilder:validation:Enum=IBGP
type OverlayProtocol string

const (
	// OverlayProtocolIBGP uses iBGP EVPN for the overlay control plane.
	OverlayProtocolIBGP OverlayProtocol = "IBGP"
)

// FabricOverlaySpec defines the overlay control-plane configuration.
// +kubebuilder:validation:XValidation:rule="self.protocol != 'IBGP' || has(self.ibgp)",message="ibgp must be set when protocol is IBGP"
type FabricOverlaySpec struct {
	// protocol is the control-plane protocol used for the EVPN overlay.
	// +required
	Protocol OverlayProtocol `json:"protocol"`

	// ibgp configures the iBGP overlay when protocol is IBGP.
	// +optional
	IBGP *FabricIBGPSpec `json:"ibgp,omitempty"`
}

// FabricIBGPSpec configures the iBGP overlay control plane.
// +kubebuilder:validation:XValidation:rule="size(self.routeReflectors) > 0",message="at least one route reflector group must be specified"
type FabricIBGPSpec struct {
	// asNumber is the BGP autonomous system number shared by all devices in the
	// iBGP fabric. Supports both plain format (1-4294967295) and dotted notation
	// (1-65535.0-65535) as per RFC 5396.
	// +required
	ASNumber intstr.IntOrString `json:"asNumber"`

	// routeReflectors lists the route reflector groups that provide iBGP scalability.
	// Each group designates a set of reflectors and their client devices.
	// +required
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	RouteReflectors []RouteReflectorGroup `json:"routeReflectors"`
}

// RouteReflectorGroup defines a set of BGP route reflectors and their clients.
type RouteReflectorGroup struct {
	// name is a unique identifier for this route reflector group within the fabric.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// deviceSelector identifies which devices act as route reflectors in this group.
	// +required
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`

	// clientDeviceSelector identifies the devices that are route reflector clients
	// for this group.
	// +required
	ClientDeviceSelector metav1.LabelSelector `json:"clientDeviceSelector"`
}

// BUMType is the mechanism used to handle BUM (Broadcast, Unknown unicast,
// Multicast) traffic in the fabric.
// +kubebuilder:validation:Enum=Multicast
type BUMType string

const (
	// BUMTypeMulticast uses PIM sparse mode for BUM traffic forwarding.
	BUMTypeMulticast BUMType = "Multicast"
)

// FabricBUMSpec defines how BUM (Broadcast, Unknown unicast, Multicast) traffic
// is forwarded across the fabric.
// +kubebuilder:validation:XValidation:rule="self.type != 'Multicast' || has(self.pim)",message="pim must be set when type is Multicast"
type FabricBUMSpec struct {
	// type selects the BUM forwarding mechanism.
	// +required
	Type BUMType `json:"type"`

	// pim configures PIM sparse mode when type is Multicast.
	// +optional
	PIM *FabricPIMSpec `json:"pim,omitempty"`
}

// FabricPIMSpec configures PIM sparse mode for BUM traffic.
type FabricPIMSpec struct {
	// anycastRendezvousPoints lists the anycast rendezvous point groups used for
	// PIM sparse mode. Anycast RPs share the same IP address across multiple
	// devices for redundancy.
	// +required
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	AnycastRendezvousPoints []AnycastRendezvousPoint `json:"anycastRendezvousPoints"`
}

// AnycastRendezvousPoint defines an anycast PIM rendezvous point group.
type AnycastRendezvousPoint struct {
	// name is a unique identifier for this rendezvous point group within the fabric.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// multicastGroups lists the multicast group address ranges served by this
	// rendezvous point. Each entry must be a valid IPv4 multicast CIDR prefix.
	// +required
	// +kubebuilder:validation:MinItems=1
	MulticastGroups []corev1alpha1.IPPrefix `json:"multicastGroups"`

	// deviceSelector identifies which devices are configured as rendezvous points
	// in this group.
	// +required
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`

	// clientDeviceSelector identifies the devices that register with the rendezvous
	// points in this group.
	// +required
	ClientDeviceSelector metav1.LabelSelector `json:"clientDeviceSelector"`
}

// FabricVTEPSpec identifies which devices act as VXLAN Tunnel Endpoints (VTEPs)
// and optionally configures their shared anycast gateway.
type FabricVTEPSpec struct {
	// deviceSelector identifies which devices are configured as VTEPs.
	// +required
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`

	// anycastGateway configures the anycast gateway shared across all VTEP devices.
	// +optional
	AnycastGateway *FabricAnycastGatewaySpec `json:"anycastGateway,omitempty"`
}

// FabricAnycastGatewaySpec configures the anycast gateway on VTEP devices.
type FabricAnycastGatewaySpec struct {
	// virtualMAC is the shared MAC address used by all anycast gateway instances
	// across the fabric. Must be a valid IEEE 802 MAC address in colon-separated
	// hexadecimal notation (e.g. f0:0c:c1:5c:00:00).
	// +required
	// +kubebuilder:validation:Pattern=`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`
	VirtualMAC string `json:"virtualMAC"`
}

// FabricStatus defines the observed state of Fabric.
type FabricStatus struct {
	// conditions represent the current state of the Fabric resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=fabrics
// +kubebuilder:resource:singular=fabric
// +kubebuilder:printcolumn:name="Underlay",type=string,JSONPath=`.spec.underlay.protocol`
// +kubebuilder:printcolumn:name="Overlay",type=string,JSONPath=`.spec.overlay.protocol`
// +kubebuilder:printcolumn:name="AS Number",type=string,JSONPath=`.spec.overlay.ibgp.asNumber`
// +kubebuilder:printcolumn:name="BUM",type=string,JSONPath=`.spec.bum.type`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Fabric is the Schema for the fabrics API
type Fabric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec FabricSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status FabricStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (f *Fabric) GetConditions() []metav1.Condition {
	return f.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (f *Fabric) SetConditions(conditions []metav1.Condition) {
	f.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// FabricList contains a list of Fabric
type FabricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Fabric `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Fabric{}, &FabricList{})
		return nil
	})
}
