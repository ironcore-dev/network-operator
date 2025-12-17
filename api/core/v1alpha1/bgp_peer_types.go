// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BGPPeerSpec defines the desired state of BGPPeer
type BGPPeerSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the BGP to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether this BGP peer is administratively up or down.
	// When Down, the BGP session with this peer is administratively shut down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// Address is the IPv4 address of the BGP peer.
	// +required
	// +kubebuilder:validation:Format=ipv4
	Address string `json:"address"`

	// ASNumber is the autonomous system number (ASN) of the BGP peer.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +required
	ASNumber intstr.IntOrString `json:"asNumber"`

	// Description is an optional human-readable description for this BGP peer.
	// This field is used for documentation purposes and may be displayed in management interfaces.
	// +optional
	Description string `json:"description,omitempty"`

	// LocalAddress specifies the local address configuration for the BGP session with this peer.
	// This determines the source address/interface for BGP packets sent to this peer.
	// +optional
	LocalAddress *BGPPeerLocalAddress `json:"localAddress,omitempty"`

	// AddressFamilies configures address family specific settings for this BGP peer.
	// Controls which address families are enabled and their specific configuration.
	// +optional
	AddressFamilies *BGPPeerAddressFamilies `json:"addressFamilies,omitempty"`
}

// BGPCommunityType represents the type of BGP community attributes that can be sent to peers.
// +kubebuilder:validation:Enum=Standard;Extended;Both
type BGPCommunityType string

const (
	// BGPCommunityTypeStandard sends only standard community attributes (RFC 1997)
	BGPCommunityTypeStandard BGPCommunityType = "Standard"
	// BGPCommunityTypeExtended sends only extended community attributes (RFC 4360)
	BGPCommunityTypeExtended BGPCommunityType = "Extended"
	// BGPCommunityTypeBoth sends both standard and extended community attributes
	BGPCommunityTypeBoth BGPCommunityType = "Both"
)

// BGPPeerLocalAddress defines the local address configuration for a BGP peer.
type BGPPeerLocalAddress struct {
	// InterfaceRef is a reference to an Interface resource whose IP address will be used
	// as the source address for BGP packets sent to this peer.
	// The Interface object must exist in the same namespace.
	// +required
	InterfaceRef LocalObjectReference `json:"interfaceRef,omitempty"`
}

// BGPPeerAddressFamilies defines the address family specific configuration for a BGP peer.
type BGPPeerAddressFamilies struct {
	// Ipv4Unicast configures IPv4 unicast address family settings for this peer.
	// Controls IPv4 unicast route exchange and peer-specific behavior.
	// +optional
	Ipv4Unicast *BGPPeerAddressFamily `json:"ipv4Unicast,omitempty"`

	// Ipv6Unicast configures IPv6 unicast address family settings for this peer.
	// Controls IPv6 unicast route exchange and peer-specific behavior.
	// +optional
	Ipv6Unicast *BGPPeerAddressFamily `json:"ipv6Unicast,omitempty"`

	// L2vpnEvpn configures L2VPN EVPN address family settings for this peer.
	// Controls EVPN route exchange and peer-specific behavior.
	// +optional
	L2vpnEvpn *BGPPeerAddressFamily `json:"l2vpnEvpn,omitempty"`
}

// BGPPeerAddressFamily defines common configuration for a BGP peer's address family.
type BGPPeerAddressFamily struct {
	// Enabled determines whether this address family is activated for this specific peer.
	// When false, the address family is not negotiated with this peer.
	// Defaults to false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SendCommunity specifies which community attributes should be sent to this BGP peer
	// for this address family. If not specified, no community attributes are sent.
	// +optional
	SendCommunity BGPCommunityType `json:"sendCommunity,omitempty"`

	// RouteReflectorClient indicates whether this peer should be treated as a route reflector client
	// for this specific address family. Defaults to false.
	// +optional
	RouteReflectorClient bool `json:"routeReflectorClient,omitempty"`
}

// BGPPeerStatus defines the observed state of BGPPeer.
type BGPPeerStatus struct {
	// SessionState is the current operational state of the BGP session.
	// +optional
	SessionState BGPPeerSessionState `json:"sessionState,omitempty"`

	// LastEstablishedTime is the timestamp when the BGP session last transitioned to the ESTABLISHED state.
	// A frequently changing timestamp indicates session instability (flapping).
	// +optional
	LastEstablishedTime *metav1.Time `json:"lastEstablishedTime,omitempty"`

	// AdvertisedPrefixesSummary provides a human-readable summary of advertised prefixes
	// across all address families (e.g., "10 (IPv4Unicast), 5 (IPv6Unicast)").
	// This field is computed by the controller from the AddressFamilies field.
	// +optional
	AdvertisedPrefixesSummary string `json:"advertisedPrefixesSummary,omitempty"`

	// AddressFamilies contains per-address-family statistics for this peer.
	// Only address families that are enabled and negotiated with the peer are included.
	// +optional
	// +listType=map
	// +listMapKey=afiSafi
	// +patchStrategy=merge
	// +patchMergeKey=afiSafi
	AddressFamilies []AddressFamilyStatus `json:"addressFamilies,omitempty"`

	// ObservedGeneration reflects the .metadata.generation that was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The conditions are a list of status objects that describe the state of the BGP.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BGPPeerSessionState represents the operational state of a BGP peer session.
// +kubebuilder:validation:Enum=Idle;Connect;Active;OpenSent;OpenConfirm;Established;Unknown
type BGPPeerSessionState string

const (
	// BGPPeerSessionStateIdle indicates the peer is down and in the idle state of the FSM.
	BGPPeerSessionStateIdle BGPPeerSessionState = "Idle"
	// BGPPeerSessionStateConnect indicates the peer is down and the session is waiting for
	// the underlying transport session to be established.
	BGPPeerSessionStateConnect BGPPeerSessionState = "Connect"
	// BGPPeerSessionStateActive indicates the peer is down and the local system is awaiting
	// a connection from the remote peer.
	BGPPeerSessionStateActive BGPPeerSessionState = "Active"
	// BGPPeerSessionStateOpenSent indicates the peer is in the process of being established.
	// The local system has sent an OPEN message.
	BGPPeerSessionStateOpenSent BGPPeerSessionState = "OpenSent"
	// BGPPeerSessionStateOpenConfirm indicates the peer is in the process of being established.
	// The local system is awaiting a NOTIFICATION or KEEPALIVE message.
	BGPPeerSessionStateOpenConfirm BGPPeerSessionState = "OpenConfirm"
	// BGPPeerSessionStateEstablished indicates the peer is up - the BGP session with the peer is established.
	BGPPeerSessionStateEstablished BGPPeerSessionState = "Established"
	// BGPPeerSessionStateUnknown indicates the peer state is unknown.
	BGPPeerSessionStateUnknown BGPPeerSessionState = "Unknown"
)

// AddressFamilyStatus defines the prefix exchange statistics for a single address family (e.g., IPv4-Unicast).
type AddressFamilyStatus struct {
	// AfiSafi identifies the address family and subsequent address family.
	// +required
	AfiSafi BGPAddressFamilyType `json:"afiSafi"`

	// AcceptedPrefixes is the number of prefixes received from the peer that have passed the inbound policy
	// and are stored in the neighbor-specific table (Adj-RIB-In).
	// +kubebuilder:validation:Minimum=0
	// +optional
	AcceptedPrefixes int64 `json:"acceptedPrefixes,omitempty"`

	// AdvertisedPrefixes is the number of prefixes currently being advertised to the peer after passing
	// the outbound policy. This reflects the state of the outbound routing table for the peer (Adj-RIB-Out).
	// +kubebuilder:validation:Minimum=0
	// +optional
	AdvertisedPrefixes int64 `json:"advertisedPrefixes,omitempty"`
}

// BGPAddressFamilyType represents the BGP address family identifier (AFI/SAFI combination).
// +kubebuilder:validation:Enum=IPv4Unicast;IPv6Unicast;L2vpnEvpn
type BGPAddressFamilyType string

const (
	// BGPAddressFamilyIpv4Unicast represents the IPv4 Unicast address family (AFI=1, SAFI=1).
	BGPAddressFamilyIpv4Unicast BGPAddressFamilyType = "IPv4Unicast"
	// BGPAddressFamilyIpv6Unicast represents the IPv6 Unicast address family (AFI=2, SAFI=1).
	BGPAddressFamilyIpv6Unicast BGPAddressFamilyType = "IPv6Unicast"
	// BGPAddressFamilyL2vpnEvpn represents the L2VPN EVPN address family (AFI=25, SAFI=70).
	BGPAddressFamilyL2vpnEvpn BGPAddressFamilyType = "L2vpnEvpn"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bgppeers
// +kubebuilder:resource:singular=bgppeer
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Session State",type=string,JSONPath=`.status.sessionState`,priority=1
// +kubebuilder:printcolumn:name="Last Established",type="date",JSONPath=`.status.lastEstablishedTime`,priority=1
// +kubebuilder:printcolumn:name="Advertised Prefixes",type=string,JSONPath=`.status.advertisedPrefixesSummary`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BGPPeer is the Schema for the bgppeers API
type BGPPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec BGPPeerSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status BGPPeerStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (bgp *BGPPeer) GetConditions() []metav1.Condition {
	return bgp.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (bgp *BGPPeer) SetConditions(conditions []metav1.Condition) {
	bgp.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// BGPPeerList contains a list of BGPPeer
type BGPPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPPeer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGPPeer{}, &BGPPeerList{})
}
