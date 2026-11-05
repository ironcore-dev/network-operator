// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bgppeerconfigs,verbs=get;list;watch

// BGPPeerConfigSpec defines the Cisco NX-OS specific BGP peer configuration.
type BGPPeerConfigSpec struct {
	// AddressFamilies configures supported BGP address families and their Cisco NX-OS specific settings.
	// +optional
	AddressFamilies *BGPPeerConfigAddressFamilies `json:"addressFamilies,omitempty"`
}

// BGPPeerConfigAddressFamilies defines the Cisco NX-OS specific configuration for a BGP peer's
// supported address families. Only l2vpn-evpn is configurable, which is enforced structurally
// by exposing solely the L2vpnEvpn field.
type BGPPeerConfigAddressFamilies struct {
	// L2vpnEvpn configures L2VPN EVPN address family support for this peer.
	// +optional
	L2vpnEvpn *BGPPeerConfigL2vpnEvpn `json:"l2vpnEvpn,omitempty"`
}

// L2vpnEvpnRewriteRouteTargetASN reports whether rewrite of the Route Target ASN is enabled
// for the l2vpn-evpn address family of this peer.
func (af *BGPPeerConfigAddressFamilies) L2vpnEvpnRewriteRouteTargetASN() bool {
	return af != nil && af.L2vpnEvpn != nil && af.L2vpnEvpn.RewriteRouteTargetASN != nil && *af.L2vpnEvpn.RewriteRouteTargetASN
}

// BGPPeerConfigL2vpnEvpn defines the Cisco NX-OS specific configuration for the
// L2VPN EVPN address family of a BGP peer.
type BGPPeerConfigL2vpnEvpn struct {
	// RewriteRouteTargetASN rewrites the AS portion of auto-generated EVPN Route Targets
	// when advertising to this peer. Only valid for l2vpn-evpn with rt auto.
	// +optional
	RewriteRouteTargetASN *bool `json:"rewriteRouteTargetASN,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=bgppeerconfigs
// +kubebuilder:resource:singular=bgppeerconfig
// +kubebuilder:resource:shortName=bgppeercfg;nxbgppeer

// BGPPeerConfig is the Schema for the bgppeerconfigs API
type BGPPeerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of BGPPeerConfig
	// +required
	Spec BGPPeerConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// BGPPeerConfigList contains a list of BGPPeerConfig
type BGPPeerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPPeerConfig `json:"items"`
}

// init registers the BGPPeerConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the BGPPeer core type.
func init() {
	v1alpha1.RegisterBGPPeerDependency(GroupVersion.WithKind("BGPPeerConfig"))
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &BGPPeerConfig{}, &BGPPeerConfigList{})
		return nil
	})
}
