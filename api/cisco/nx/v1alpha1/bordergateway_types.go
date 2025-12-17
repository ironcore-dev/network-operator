// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// BorderGatewaySpec defines the desired state of BorderGateway
type BorderGatewaySpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef v1alpha1.LocalObjectReference `json:"deviceRef"`

	// AdminState indicates whether the BorderGateway instance is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState v1alpha1.AdminState `json:"adminState,omitempty"`

	// MultisiteID is the identifier for the multisite border gateway.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=281474976710655
	// +kubebuilder:validation:ExclusiveMaximum=false
	MultisiteID int64 `json:"multisiteId"`

	// SourceInterfaceRef is a reference to the loopback interface used as the source for the
	// border gateway virtual IP address. A best practice is to use a separate loopback address
	// for the NVE source interface and multi-site source interface. The loopback interface must
	// be configured with a /32 IPv4 address. This /32 IP address needs be known by the transient
	// devices in the transport network and the remote VTEPs.
	// +required
	SourceInterfaceRef v1alpha1.LocalObjectReference `json:"sourceInterfaceRef"`

	// DelayRestoreTime specifies the time to wait before restoring EVPN multisite border gateway
	// functionality after a failure. This allows time for the network to stabilize before resuming
	// traffic forwarding across sites.
	// +optional
	// +kubebuilder:default="180s"
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	DelayRestoreTime metav1.Duration `json:"delayRestoreTime"`

	// InterconnectInterfaceRefs is a list of interfaces that provide connectivity to the border gateway.
	// Each interface can be configured with object tracking to monitor its availability.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	InterconnectInterfaceRefs []InterconnectInterfaceReference `json:"interconnectInterfaceRefs,omitempty"`

	// BGPPeerRefs is a list of BGP peers that are part of the border gateway configuration.
	// Each peer can be configured with a peer type to specify its role in the EVPN multisite topology.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	BGPPeerRefs []BGPPeerReference `json:"bgpPeerRefs,omitempty"`

	// StormControl is the storm control configuration for the border gateway, allowing to rate-limit
	// BUM (Broadcast, Unknown unicast, Multicast) traffic on the border gateway interface.
	// +optional
	// +listType=map
	// +listMapKey=traffic
	// +kubebuilder:validation:MinItems=1
	StormControl []StormControl `json:"stormControl,omitempty"`
}

// InterconnectInterfaceReference defines an interface used for border gateway interconnectivity
// with optional object tracking configuration.
type InterconnectInterfaceReference struct {
	v1alpha1.LocalObjectReference `json:",inline"`

	// Tracking specifies the EVPN multisite tracking mode for this interconnect interface.
	// +required
	Tracking InterconnectTrackingType `json:"tracking"`
}

// InterconnectTrackingType defines the tracking mode for border gateway interconnect interfaces.
// +kubebuilder:validation:Enum=DataCenterInterconnect;Fabric
type InterconnectTrackingType string

const (
	// InterconnectTrackingTypeDCI represents Data Center Interconnect tracking mode.
	// Used for interfaces connecting to remote data centers.
	InterconnectTrackingTypeDCI InterconnectTrackingType = "DataCenterInterconnect"

	// InterconnectTrackingTypeFabric represents Fabric tracking mode.
	// Used for interfaces connecting to the local fabric.
	InterconnectTrackingTypeFabric InterconnectTrackingType = "Fabric"
)

// BGPPeerReference defines a BGP peer used for border gateway with peer type configuration.
type BGPPeerReference struct {
	v1alpha1.LocalObjectReference `json:",inline"`

	// PeerType specifies the role of this BGP peer in the EVPN multisite topology.
	// FabricExternal is used for peers outside the fabric, while FabricBorderLeaf is used
	// for border leaf peers within the fabric.
	// +required
	PeerType BGPPeerType `json:"peerType,omitempty"`
}

// BGPPeerType defines the peer type for border gateway BGP peers.
// +kubebuilder:validation:Enum=FabricExternal;FabricBorderLeaf
type BGPPeerType string

const (
	// BGPPeerTypeFabricExternal represents a BGP peer outside the fabric.
	// Used for external peers in EVPN multisite configurations.
	BGPPeerTypeFabricExternal BGPPeerType = "FabricExternal"

	// BGPPeerTypeFabricBorderLeaf represents a BGP peer that is a border leaf within the fabric.
	// Used for border leaf peers in EVPN multisite configurations.
	BGPPeerTypeFabricBorderLeaf BGPPeerType = "FabricBorderLeaf"
)

type StormControl struct {
	// Level is the suppression level as a percentage of the interface bandwidth.
	// Must be a floating point number between 1.0 and 100.0.
	// +required
	// +kubebuilder:validation:Pattern=`^([1-9][0-9]?(\.[0-9]+)?|100(\.0+)?)$`
	Level string `json:"level"`

	// Traffic specifies the type of BUM traffic the storm control applies to.
	// +required
	Traffic TrafficType `json:"traffic"`
}

// TrafficType defines the type of traffic for storm control.
// +kubebuilder:validation:Enum=Broadcast;Multicast;Unicast
type TrafficType string

const (
	// TrafficTypeBroadcast represents broadcast traffic.
	TrafficTypeBroadcast TrafficType = "Broadcast"
	// TrafficTypeMulticast represents multicast traffic.
	TrafficTypeMulticast TrafficType = "Multicast"
	// TrafficTypeUnicast represents unicast traffic.
	TrafficTypeUnicast TrafficType = "Unicast"
)

// BorderGatewayStatus defines the observed state of BorderGateway.
type BorderGatewayStatus struct {
	// The conditions are a list of status objects that describe the state of the Banner.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bordergateways
// +kubebuilder:resource:singular=bordergateway
// +kubebuilder:resource:shortName=bgw
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BorderGateway is the Schema for the bordergateways API
type BorderGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec BorderGatewaySpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status BorderGatewayStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (bgw *BorderGateway) GetConditions() []metav1.Condition {
	return bgw.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (bgw *BorderGateway) SetConditions(conditions []metav1.Condition) {
	bgw.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// BorderGatewayList contains a list of BorderGateway
type BorderGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []BorderGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BorderGateway{}, &BorderGatewayList{})
}
