// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vtepconfigs,verbs=get;list;watch

// VTEPConfig defines the Cisco-specific configuration of a VTEP/NVE
type VTEPConfigSpec struct {
	// AdvertiseVMAC is equivalent to: `advertise virtual-rmac`
	// +optional
	AdvertiseVMAC *bool `json:"advertiseVMAC,omitempty"`

	// HoldDownTime is equivalent to: `source-interface hold-down-time
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1500
	HoldDownTime uint16 `json:"holdDownTime,omitempty"`

	// InfraVLANs is the list of infrastructure VLANs for ingress replication, equivalent to: `system nve infra-vlan`
	// See admission webhook for additional validation rules.
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
	ID uint `json:"id,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3967
	RangeMin uint `json:"rangeMin,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3967
	RangeMax uint `json:"rangeMax,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vtepconfigs
// +kubebuilder:resource:singular=vtepconfig

// VTEPConfig is the Schema for the VTEP API
type VTEPConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of VTEP
	// +required
	Spec VTEPConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// VTEPList contains a list of VTEP
type VTEPConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VTEPConfig `json:"items"`
}

// init registers the VTEPConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the VTEP core type.
func init() {
	corev1alpha1.RegisterVTEPDependency(GroupVersion.WithKind("VTEPConfig"))
	SchemeBuilder.Register(&VTEPConfig{}, &VTEPConfigList{})
}
