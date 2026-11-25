// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// ExternalPeeringSpec defines the desired state of ExternalPeering
type ExternalPeeringSpec struct {
	// deviceSelector identifies which devices are members of this external peering.
	// All devices whose labels match this selector will be enrolled.
	// +required
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`

	// routingDomainRef references the [RoutingDomain] used by this external peering.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="routingDomainRef is immutable"
	RoutingDomainRef corev1alpha1.LocalObjectReference `json:"routingDomainRef"`

	// peering is the specification of the bgp session attributes.
	// +required
	Peering Peering `json:"peering"`
}

type Peering struct {
	// sourceAddress specifies the source address configuration for the BGP session.
	// +required
	SourceAddress SourceAddress `json:"sourceAddress"`

	// localASNumber is the autonomous system number (ASN) for the BGP router created for this external peering.
	// Supports both plain format (1-4294967295) and dotted notation (0-65535.0-65535) as per RFC 5396.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="localASNumber is immutable"
	LocalASNumber intstr.IntOrString `json:"localASNumber"`

	// remoteASNumber is the autonomous system number (ASN) of the external BGP peer.
	// Supports both plain format (1-4294967295) and dotted notation (0-65535.0-65535) as per RFC 5396.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="remoteASNumber is immutable"
	RemoteASNumber intstr.IntOrString `json:"remoteASNumber"`

	// listenRange is the IP address (range) of the external BGP peer.
	// If a single IP address is specified, it should be in given with a /32 CIDR notation.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="listenRange is immutable"
	ListenRange corev1alpha1.IPPrefix `json:"listenRange"`

	// AddressFamilies configures address family specific settings for this BGP peer.
	// Controls which address families are enabled and their specific configuration.
	// +optional
	AddressFamilies *BGPPeerAddressFamilies `json:"addressFamilies,omitempty"`

	// LocalAS configures the local AS number and how it factors into BGP announcements for this peer.
	// +optional
	LocalAS *corev1alpha1.LocalAS `json:"localAS,omitempty"`

	// TTL sets the time-to-live for eBGP sessions that span multiple hops.
	// When set, allows establishing eBGP sessions with non-directly-connected peers
	// by setting the IP TTL of outgoing BGP packets to this value.
	// When unset, eBGP multihop is disabled (equivalent to TTL=1 — directly connected peers only).
	// +optional
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=255
	TTL *int32 `json:"ttl,omitempty"`

	// BFD defines the Bidirectional Forwarding Detection configuration for the interface.
	// BFD is only applicable for Layer 3 interfaces.
	// +optional
	BFD *corev1alpha1.BFD `json:"bfd,omitempty"`
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

	// InboundRoutingPolicy defines a RoutingPolicy applied to routes received from this peer
	// for this address family.
	// +optional
	InboundRoutingPolicy *RoutingPolicy `json:"inboundRoutingPolicy,omitempty"`

	// OutboundRoutingPolicy defines a RoutingPolicy applied to routes advertised to this peer
	// for this address family.
	// +optional
	OutboundRoutingPolicy *RoutingPolicy `json:"outboundRoutingPolicy,omitempty"`
}

type RoutingPolicy struct {
	// Template defines the template for the RoutingPolicy.
	// +required
	Template RoutingPolicyTemplate `json:"template"`
}

type RoutingPolicyTemplate struct {
	// Spec defines the desired state of the RoutingPolicy.
	// +required
	Spec RoutingPolicySpec `json:"spec"`
}

// RoutingPolicySpec defines the desired state of RoutingPolicy
type RoutingPolicySpec struct {
	// Name is the identifier of the RoutingPolicy on the device.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// A list of policy statements to apply.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Statements []corev1alpha1.PolicyStatement `json:"statements"`
}

// SourceAddressType defines the type of source address used for the BGP session.
// It can only be "Loopback".
// +kubebuilder:validation:Enum=Loopback
type SourceAddressType string

const (
	SourceAddressTypeLoopback SourceAddressType = "Loopback"
)

type SourceAddress struct {
	// type is the type of source address used for the BGP session.
	// +required
	Type SourceAddressType `json:"type"`

	// Name is the name of the interface.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// ipAddressPoolRef references the [IPAddressPool] used for allocating interfaces addresses on the device(s).
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ipAddressPoolRef is immutable"
	IPAddressPoolRef corev1alpha1.LocalObjectReference `json:"ipAddressPoolRef"`
}

// ExternalPeeringStatus defines the observed state of ExternalPeering.
type ExternalPeeringStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ExternalPeering resource.
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
// +kubebuilder:resource:path=externalpeerings
// +kubebuilder:resource:singular=externalpeering
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ExternalPeering is the Schema for the externalpeerings API
type ExternalPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ExternalPeeringSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ExternalPeeringStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (p *ExternalPeering) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *ExternalPeering) SetConditions(conds []metav1.Condition) {
	p.Status.Conditions = conds
}

// +kubebuilder:object:root=true

// ExternalPeeringList contains a list of ExternalPeering
type ExternalPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ExternalPeering `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ExternalPeering{}, &ExternalPeeringList{})
		return nil
	})
}
