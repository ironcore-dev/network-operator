// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1alpha "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VTEPConfig defines the Cisco-specific configuration of a VTEP/NVE
type VTEPConfigSpec struct {
	// AdvertiseVMAC is equivalent to: `advertise virtual-rmac`
	// +optional
	AdvertiseVMAC *bool `json:"advertiseVMAC,omitempty"`

	// HoldDownTime is equivalent to: `source-interface hold-down-time`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1500
	HoldDownTime uint16 `json:"holdDownTime,omitempty"`

	// InfraVLANs is the list of infrastructure VLANs for ingress replication, equivalent to: `system VTEP infra-vlan`
	// +optional
	// +kubebuilder:validation:MaxItems=2
	// TODO: admission webhook to validate:
	// - no overlapping ranges
	// - range of vlans configured does must not exceed 512
	InfraVLANs []VLANListItem `json:"infraVLANs,omitempty"`
}

// VLANListItem represents a single VLAN ID or a range start-end (e.g. "100", "200-300").
// +kubebuilder:validation:Pattern=`^(\d{1,4}(-\d{1,4})?)$`
// +kubebuilder:validation:XValidation:rule="self.contains('-') ? (int(self.split('-')[0]) >= 1 && int(self.split('-')[0]) <= 3967 && int(self.split('-')[1]) >= 1 && int(self.split('-')[1]) <= 3967 && int(self.split('-')[1]) > int(self.split('-')[0])) : (int(self) >= 1 && int(self) <= 3967)",message="Single VLAN 1-3967; range both 1-3967 and end > start"
type VLANListItem string

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
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

func init() {
	corev1alpha.RegisterVTEPDependency(GroupVersion.WithKind("VTEPConfig"))
	SchemeBuilder.Register(&VTEPConfig{}, &VTEPConfigList{})
}
