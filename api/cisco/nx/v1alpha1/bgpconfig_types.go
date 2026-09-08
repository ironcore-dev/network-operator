// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bgpconfigs,verbs=get;list;watch

// BGPConfigSpec defines the Cisco NX-OS specific BGP configuration.
type BGPConfigSpec struct {
	// AddressFamilies configures supported BGP address families and their Cisco NX-OS specific settings.
	// +optional
	AddressFamilies *BGPConfigAddressFamilies `json:"addressFamilies,omitempty"`
}

// BGPConfigAddressFamilies defines the Cisco NX-OS specific configuration for supported BGP address families.
type BGPConfigAddressFamilies struct {
	// L2vpnEvpn configures L2VPN EVPN address family support.
	// +optional
	L2vpnEvpn *BGPL2vpnEvpn `json:"l2vpnEvpn,omitempty"`

	// Ipv4Unicast configures specific IPv4 unicast address family settings.
	// +optional
	Ipv4Unicast *BGPConfigUnicastAddressFamily `json:"ipv4Unicast,omitempty"`

	// Ipv6Unicast configures specific IPv6 unicast address family settings.
	// +optional
	Ipv6Unicast *BGPConfigUnicastAddressFamily `json:"ipv6Unicast,omitempty"`
}

// Ipv4UnicastExportGatewayIP reports whether export gateway IP is enabled for the IPv4 unicast address family.
func (af *BGPConfigAddressFamilies) Ipv4UnicastExportGatewayIP() bool {
	return af != nil && af.Ipv4Unicast != nil && af.Ipv4Unicast.ExportGatewayIP != nil && *af.Ipv4Unicast.ExportGatewayIP
}

// Ipv6UnicastExportGatewayIP reports whether export gateway IP is enabled for the IPv6 unicast address family.
func (af *BGPConfigAddressFamilies) Ipv6UnicastExportGatewayIP() bool {
	return af != nil && af.Ipv6Unicast != nil && af.Ipv6Unicast.ExportGatewayIP != nil && *af.Ipv6Unicast.ExportGatewayIP
}

// Ipv4UnicastAdvertiseL2vpnEvpn reports whether advertise l2vpn evpn is enabled for the IPv4 unicast address family.
func (af *BGPConfigAddressFamilies) Ipv4UnicastAdvertiseL2vpnEvpn() bool {
	return af != nil && af.Ipv4Unicast != nil && af.Ipv4Unicast.AdvertiseL2vpnEvpn != nil && *af.Ipv4Unicast.AdvertiseL2vpnEvpn
}

// Ipv6UnicastAdvertiseL2vpnEvpn reports whether advertise l2vpn evpn is enabled for the IPv6 unicast address family.
func (af *BGPConfigAddressFamilies) Ipv6UnicastAdvertiseL2vpnEvpn() bool {
	return af != nil && af.Ipv6Unicast != nil && af.Ipv6Unicast.AdvertiseL2vpnEvpn != nil && *af.Ipv6Unicast.AdvertiseL2vpnEvpn
}

// BGPL2vpnEvpn defines the configuration for L2VPN EVPN address family.
type BGPL2vpnEvpn struct {
	// AdvertisePIP controls whether the BGP EVPN address-family should advertise the primary IP address (PIP) as the next-hop
	// when advertising prefix routes or loopback interface routes in BGP on vPC enabled leaf or border leaf switches.
	// +optional
	// +kubebuilder:default=false
	AdvertisePIP bool `json:"advertisePIP,omitempty"`
}

// BGPConfigUnicastAddressFamily defines specific configuration shared
// across unicast BGP address families.
type BGPConfigUnicastAddressFamily struct {
	// ExportGatewayIP enables advertising the gateway IP in EVPN Type-5 routes,
	// required for symmetric IRB in VXLAN BGP EVPN topologies.
	// +optional
	ExportGatewayIP *bool `json:"exportGatewayIP,omitempty"`

	// AdvertiseL2vpnEvpn enables advertising EVPN routes within the VRF
	// unicast address family. Required on border gateway nodes for cross-site
	// EVPN Type-5 IP prefix route advertisement.
	// +optional
	AdvertiseL2vpnEvpn *bool `json:"advertiseL2vpnEvpn,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=bgpconfigs
// +kubebuilder:resource:singular=bgpconfig
// +kubebuilder:resource:shortName=nxbgp

// BGPConfig is the Schema for the bgpconfigs API
type BGPConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of BGPConfig
	// +required
	Spec BGPConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// BGPConfigList contains a list of BGPConfig
type BGPConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPConfig `json:"items"`
}

// init registers the BGPConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the BGP core type.
func init() {
	v1alpha1.RegisterBGPDependency(GroupVersion.WithKind("BGPConfig"))
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &BGPConfig{}, &BGPConfigList{})
		return nil
	})
}
