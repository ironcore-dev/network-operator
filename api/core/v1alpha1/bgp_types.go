// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BGPSpec defines the desired state of BGP
type BGPSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the BGP to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether this BGP router is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// ASNumber is the autonomous system number (ASN) for the BGP router.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +required
	ASNumber intstr.IntOrString `json:"asNumber"`

	// RouterID is the BGP router identifier, used in BGP messages to identify the originating router.
	// Follows dotted quad notation (IPv4 format).
	// +required
	// +kubebuilder:validation:Format=ipv4
	RouterID string `json:"routerId"`

	// AddressFamilies configures supported BGP address families and their specific settings.
	// +optional
	AddressFamilies *BGPAddressFamilies `json:"addressFamilies,omitempty"`
}

// BGPMultipath defines the configuration for BGP multipath behavior.
type BGPMultipath struct {
	// Enabled determines whether BGP is allowed to use multiple paths for forwarding.
	// When false, BGP will only use a single best path regardless of multiple equal-cost paths.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Ebgp configures multipath behavior for external BGP (eBGP) paths.
	// +optional
	Ebgp *BGPMultipathEbgp `json:"ebgp,omitempty"`

	// Ibgp configures multipath behavior for internal BGP (iBGP) paths.
	// +optional
	Ibgp *BGPMultipathIbgp `json:"ibgp,omitempty"`
}

// BGPMultipathEbgp defines the configuration for eBGP multipath behavior.
type BGPMultipathEbgp struct {
	// AllowMultipleAs enables the use of multiple paths with different AS paths for eBGP.
	// When true, relaxes the requirement that multipath candidates must have identical AS paths.
	// This corresponds to the "RelaxAs" mode.
	// +optional
	AllowMultipleAs bool `json:"allowMultipleAs,omitempty"`

	// MaximumPaths sets the maximum number of eBGP paths that can be used for multipath load balancing.
	// Valid range is 1-64 when specified. When omitted, no explicit limit is configured.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	MaximumPaths int8 `json:"maximumPaths,omitempty"`
}

// BGPMultipathIbgp defines the configuration for iBGP multipath behavior.
type BGPMultipathIbgp struct {
	// MaximumPaths sets the maximum number of iBGP paths that can be used for multipath load balancing.
	// Valid range is 1-64 when specified. When omitted, no explicit limit is configured.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	MaximumPaths int8 `json:"maximumPaths,omitempty"`
}

// BGPAddressFamilies defines the configuration for supported BGP address families.
type BGPAddressFamilies struct {
	// Ipv4Unicast configures IPv4 unicast address family support.
	// Enables exchange of IPv4 unicast routes between BGP peers.
	// +optional
	Ipv4Unicast *BGPAddressFamily `json:"ipv4Unicast,omitempty"`

	// Ipv6Unicast configures IPv6 unicast address family support.
	// Enables exchange of IPv6 unicast routes between BGP peers.
	// +optional
	Ipv6Unicast *BGPAddressFamily `json:"ipv6Unicast,omitempty"`

	// L2vpnEvpn configures L2VPN EVPN address family support.
	// Enables exchange of Ethernet VPN routes for overlay network services.
	// +optional
	L2vpnEvpn *BGPL2vpnEvpn `json:"l2vpnEvpn,omitempty"`
}

// BGPAddressFamily defines common configuration for a BGP address family.
type BGPAddressFamily struct {
	// Enabled determines whether this address family is activated for BGP sessions.
	// When false, the address family is not negotiated with peers.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Multipath configures address family specific multipath behavior.
	// When specified, overrides global multipath settings for this address family.
	// +optional
	Multipath *BGPMultipath `json:"multipath,omitempty"`
}

// BGPL2vpnEvpn defines the configuration for L2VPN EVPN address family.
type BGPL2vpnEvpn struct {
	BGPAddressFamily `json:",inline"`

	// RouteTargetPolicy configures route target filtering behavior for EVPN routes.
	// Controls which routes are retained based on route target matching.
	// +optional
	RouteTargetPolicy *BGPRouteTargetPolicy `json:"routeTargetPolicy,omitempty"`
}

// BGPRouteTargetPolicy defines the policy for route target filtering in EVPN.
type BGPRouteTargetPolicy struct {
	// RetainAll controls whether all route targets are retained regardless of import policy.
	// +optional
	RetainAll bool `json:"retainAll,omitempty"`
}

// BGPStatus defines the observed state of BGP.
type BGPStatus struct {
	// The conditions are a list of status objects that describe the state of the BGP.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bgp
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BGP is the Schema for the bgp API
type BGP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec BGPSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status BGPStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (bgp *BGP) GetConditions() []metav1.Condition {
	return bgp.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (bgp *BGP) SetConditions(conditions []metav1.Condition) {
	bgp.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// BGPList contains a list of BGP
type BGPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGP{}, &BGPList{})
}
