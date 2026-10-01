// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=ethernetsegmentconfigs,verbs=get;list;watch

// EthernetSegmentConfigSpec defines the Cisco NX-OS-specific configuration of an EthernetSegment.
type EthernetSegmentConfigSpec struct {
	// FRRAnycastSourceIP sets the EVPN ES fast-reroute anycast source IP
	// (evpn esi multihoming / frr-anycast-src-ip). It is applied to the device-global
	// EVPN multihoming configuration. Omit to leave it unmanaged.
	// +optional
	// +kubebuilder:validation:Format=ip
	FRRAnycastSourceIP string `json:"frrAnycastSourceIP,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ethernetsegmentconfigs
// +kubebuilder:resource:singular=ethernetsegmentconfig
// +kubebuilder:resource:shortName=nxes

// EthernetSegmentConfig is the Schema for the ethernetsegmentconfigs API
type EthernetSegmentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of the EthernetSegmentConfig
	// +required
	Spec EthernetSegmentConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// EthernetSegmentConfigList contains a list of EthernetSegmentConfig
type EthernetSegmentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EthernetSegmentConfig `json:"items"`
}

// init registers the EthernetSegmentConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the EthernetSegment core type.
func init() {
	v1alpha1.RegisterEthernetSegmentDependency(GroupVersion.WithKind("EthernetSegmentConfig"))
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &EthernetSegmentConfig{}, &EthernetSegmentConfigList{})
		return nil
	})
}
