// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NVESpec defines the desired state of NVE, a Cisco NX-OS specific VTEP configuration. Uses always ID '1'.
type NVESpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef corev1alpha1.LocalObjectReference `json:"deviceRef"`

	// AdvertiseVMAC is equivalent to: `advertise virtual-rmac`
	// +optional
	AdvertiseVMAC *bool `json:"advertiseVMAC,omitempty"`

	// HoldDownTime is equivalent to: `source-interface hold-down-time`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1500
	HoldDownTime uint16 `json:"holdDownTime,omitempty"`

	// StormControl defines storm-control parameters for the NVE interface.
	// TODO: may be this can be moved to provider-specific config of the evpn-control-plane?
	// +optional
	StormControl *StormControl `json:"stormControl,omitempty"`

	// InfraVLANs is the list of infrastructure VLANs for ingress replication, equivalent to: `system nve infra-vlan`
	// +optional
	// +kubebuilder:validation:MaxItems=2
	// TODO: admission webhook to validate:
	// - no overlapping ranges
	// - range of vlans configured does must not exceed 512
	InfraVLANs []VLANListItem `json:"infraVLANs,omitempty"`

	// GlobalMulticastGroups defines the multicast distribution tree
	// TODO: move to a cisco-specific EVPNControlPlane?
	// +optional
	GlobalMulticastGroups *GlobalMulticastConfig `json:"globalMulticastGroups,omitempty"`

	// MultiSite defines EVPN Multisite Border Gateway configuration.
	// TODO: may be we want to move this to cisco-specific EVPNControlPlane
	// +optional
	MultiSite *MultiSiteConfig `json:"multisite,omitempty"`
}

// VLANListItem represents a single VLAN ID or a range start-end (e.g. "100", "200-300").
// +kubebuilder:validation:Pattern=`^(\d{1,4}(-\d{1,4})?)$`
// +kubebuilder:validation:XValidation:rule="self.contains('-') ? (int(self.split('-')[0]) >= 1 && int(self.split('-')[0]) <= 3967 && int(self.split('-')[1]) >= 1 && int(self.split('-')[1]) <= 3967 && int(self.split('-')[1]) > int(self.split('-')[0])) : (int(self) >= 1 && int(self) <= 3967)",message="Single VLAN 1-3967; range both 1-3967 and end > start"
type VLANListItem string

type StormControl struct {
	// Unicast is a sorm-control configuration parameter, as percentage of port capacity (an integer value).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Unicast *uint32 `json:"unicast,omitempty"`

	// Multicast is a sorm-control configuration parameter, as percentage of port capacity (an integer value).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Multicast *uint32 `json:"multicast,omitempty"`

	// Broadcast is a sorm-control configuration parameter, as percentage of port capacity (an integer value).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Broadcast *uint32 `json:"broadcast,omitempty"`
}

// GlobalMulticastConfig defines the global multicast group configuration for NVEs.
// It can be used to set the multicast groups for L2 and L3 VNIs (mutually exclusive).
// +kubebuilder:validation:XValidation:rule="!(has(self.l2) && has(self.l3))",message="l2 and l3 are mutually exclusive"
type GlobalMulticastConfig struct {
	// L2 configures the group for the Layer 2 VNI, equiv. to `global mcast-group ... L2`
	// +optional
	L2 *corev1alpha1.IPPrefix `json:"l2,omitempty"`
	// L3 configures the global multicast group for the Layer 3 VNI, equiv. to `global mcast-group ... L3`
	// +optional
	L3 *corev1alpha1.IPPrefix `json:"l3,omitempty"`
}

// MultiSiteConfig defines the EVPN Multisite Border Gateway configuration.
type MultiSiteConfig struct {
	// Enabled enables EVPN Multisite Border Gateway functionality.
	// +required
	Enabled bool `json:"enabled"`

	// ID is the site ID or unique identifier for the site in a multisite EVPN deployment.
	// equiv. to: `evpn multisite border-gateway <id>`
	// +required
	ID uint32 `json:"ID"`

	// BorderGatewayInterfaceRef is the reference to the interface used as Border Gateway Interface
	// equiv. to.:
	// ```
	// interface nve1
	// multisite border-gateway interface <interface-name>
	// ```
	// +required
	BorderGatewayInterfaceRef corev1alpha1.LocalObjectReference `json:"borderGatewayInterfaceRef"`
}

// NVEStatus defines the observed state of NVE.
type NVEStatus struct {
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=nves,shortName=nve
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NVE is the Schema for the NVE API
type NVE struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NVE
	// +required
	Spec NVESpec `json:"spec"`

	// status defines the observed state of NVE
	// +optional
	Status NVEStatus `json:"status,omitempty,omitzero"`
}

func (v *NVE) GetConditions() []metav1.Condition {
	return v.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (v *NVE) SetConditions(conditions []metav1.Condition) {
	v.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// NVEList contains a list of NVE
type NVEList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NVE `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NVE{}, &NVEList{})
}
