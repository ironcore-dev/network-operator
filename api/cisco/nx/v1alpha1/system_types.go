// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SystemSpec defines the desired state of System
type SystemSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef v1alpha1.LocalObjectReference `json:"deviceRef"`

	// JumboMtu defines the system-wide jumbo MTU setting.
	// Valid values are from 1501 to 9216.
	// +optional
	// +kubebuilder:validation:Minimum=1501
	// +kubebuilder:validation:Maximum=9216
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=9216
	JumboMTU int16 `json:"jumboMtu"`

	// ReservedVlan specifies the VLAN ID to be reserved for system use.
	// Valid values are from 1 to 4032.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4032
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=3968
	ReservedVlan int16 `json:"reservedVlan"`

	// VlanLongName enables or disables 128-character VLAN names
	// Disabled by default.
	// +optional
	// +kubebuilder:default=false
	VlanLongName bool `json:"vlanLongName"`

	// RelayServerIDOverrideDisable enables the global command
	// "ip dhcp relay information option server-id-override-disable".
	//
	// Deprecated: use dhcp.relayServerIdOverrideDisable instead. This field is kept
	// only for migration and will be removed in a future release. While both are set,
	// the setting is enabled if either is true.
	// +optional
	// +kubebuilder:default=false
	RelayServerIDOverrideDisable bool `json:"relayServerIdOverrideDisable"`

	// DHCP groups DHCP-related system settings.
	// +optional
	// +kubebuilder:default={relayServerIdOverrideDisable:false}
	DHCP DHCP `json:"dhcp,omitzero"`
}

// DHCP groups DHCP-related system settings. These require the dhcp feature,
// which the operator enables automatically when a setting here is active.
type DHCP struct {
	// RelayServerIDOverrideDisable enables the global command
	// "ip dhcp relay information option server-id-override-disable".
	// It prevents the DHCP relay agent from overriding the Server-ID (Option 54) in
	// relayed requests, which otherwise causes DHCP servers to drop the request.
	// Disabled by default. Enabling it activates the dhcp feature automatically.
	// +optional
	// +kubebuilder:default=false
	RelayServerIDOverrideDisable bool `json:"relayServerIdOverrideDisable"`
}

// SystemStatus defines the observed state of System.
type SystemStatus struct {
	// The conditions are a list of status objects that describe the state of the Banner.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=systems
// +kubebuilder:resource:singular=system
// +kubebuilder:resource:shortName=nxsystem
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// System is the Schema for the systems API
type System struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec SystemSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status SystemStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (s *System) GetConditions() []metav1.Condition {
	return s.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (s *System) SetConditions(conditions []metav1.Condition) {
	s.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// SystemList contains a list of System
type SystemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []System `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &System{}, &SystemList{})
		return nil
	})
}
