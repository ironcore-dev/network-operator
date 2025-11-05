// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=nveconfigs,verbs=get;list;watch

// NVEConfig defines the Cisco-specific configuration of a Network Virtualization Object
type NVEConfigSpec struct {
	// AdvertiseVirtualMAC controls if the NVE should advertise a virtual MAC address
	// +optional
	// +kubebuilder:default=false
	AdvertiseVirtualMAC bool `json:"advertiseVirtualMAC,omitempty"`

	// HoldDownTime defines the duration for which the switch suppresses the advertisement of the NVE loopback address.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1500
	// +kubebuilder:default=180
	HoldDownTime uint16 `json:"holdDownTime,omitempty"`

	// InfraVLANs specifies VLANs used by all SVI interfaces for uplink and vPC peer-links in VXLAN as infra-VLANs.
	// The total number of VLANs configured must not exceed 512.
	// Elements in the list must not overlap with each other.
	// +optional
	// +kubebuilder:validation:MaxItems=10
	InfraVLANs []VLANListItem `json:"infraVLANs,omitempty"`
}

// VLANListItem represents a single VLAN ID or a range start-end. If ID is set, rangeMin and rangeMax must be absent. If ID is absent, both rangeMin
// and rangeMax must be set.
// +kubebuilder:validation:XValidation:rule="!has(self.rangeMax) || self.rangeMax > self.rangeMin",message="rangeMax must be greater than rangeMin"
// +kubebuilder:validation:XValidation:rule="has(self.id) || (has(self.rangeMin) && has(self.rangeMax))",message="either ID or both rangeMin and rangeMax must be set"
// +kubebuilder:validation:XValidation:rule="!has(self.id) || (!has(self.rangeMin) && !has(self.rangeMax))",message="rangeMin and rangeMax must be omitted when ID is set"
type VLANListItem struct {
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3967
	ID uint16 `json:"id,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3967
	RangeMin uint16 `json:"rangeMin,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3967
	RangeMax uint16 `json:"rangeMax,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=nveconfigs
// +kubebuilder:resource:singular=nveconfig

// NVEConfig is the Schema for the NVE API
type NVEConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NVE
	// +required
	Spec NVEConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// NVEList contains a list of NVE
type NVEConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NVEConfig `json:"items"`
}

// init registers the NVEConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the NVE core type.
func init() {
	v1alpha1.RegisterNVEDependency(GroupVersion.WithKind("NVEConfig"))
	SchemeBuilder.Register(&NVEConfig{}, &NVEConfigList{})
}
