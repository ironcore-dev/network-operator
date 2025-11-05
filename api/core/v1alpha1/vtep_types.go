// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VTEPSpec defines the desired state of a VXLAN Tunnel Endoint (VTEP)
// +kubebuilder:validation:XValidation:rule="self.anycastInterfaceRef.name != self.primaryInterfaceRef.name",message="anycastInterfaceRef.name must differ from primaryInterfaceRef.name"
type VTEPSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration for this VTEP.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Enable indicates whether the interface is administratively up or down.
	// +required
	Enabled bool `json:"enabled"`

	// PrimaryInterfaceRef is the reference to the loopback interface used for the primary VTEP IP address.
	// +required
	PrimaryInterfaceRef LocalObjectReference `json:"primaryInterfaceRef"`

	// AnycastInterfaceRef is the reference to the loopback interface used for anycast VTEP IP address.
	// +required
	AnycastInterfaceRef LocalObjectReference `json:"anycastInterfaceRef,omitempty"`

	// SuppressARP indicates whether ARP suppression is enabled for this VTEP.
	// +required
	SuppressARP bool `json:"suppressARP"`

	// HostReachability specifies the method used for host reachability.
	// +required
	// +kubebuilder:validation:Enum=FloodAndLearn;BGP
	HostReachability HostReachabilityType `json:"hostReachability"`

	// MulticastGroup specifies the multicast group configuration for the VTEP.
	// +optional
	MulticastGroup *MulticastGroup `json:"multicastGroup,omitempty"`
}

// HostReachabilityType defines the method used for host reachability.
type HostReachabilityType string

const (
	HostReachabilityTypeBGP           HostReachabilityType = "BGP"
	HostReachabilityTypeFloodAndLearn HostReachabilityType = "FloodAndLearn"
)

type MulticastGroup struct {
	// Type specifies the multicast group level (L2 or L3).
	// +required
	// +kubebuilder:validation:Enum=L2;L3
	Type MulticastGroupType `json:"type"`

	// Prefix is the multicast group IP prefix.
	// +required
	Prefix IPPrefix `json:"prefix"`
}

// MulticastGroupType defines the type of multicast group (L2 or L3).
type MulticastGroupType string

const (
	MulticastGroupTypeL2 MulticastGroupType = "L2"
	MulticastGroupTypeL3 MulticastGroupType = "L3"
)

// VTEPStatus defines the observed state of VTEP.
type VTEPStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the VTEP resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The conditions are a list of status objects that describe the state of the VTEP.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vteps
// +kubebuilder:resource:singular=vtep
// +kubebuilder:printcolumn:name="VTEP",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VTEP is the Schema for the vteps API
type VTEP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec VTEPSpec `json:"spec"`

	// +optional
	Status VTEPStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (in *VTEP) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (in *VTEP) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// VTEPList contains a list of VTEP
type VTEPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VTEP `json:"items"`
}

var (
	// VTEPDependencies holds a registry of external provider-specific configuration types
	// that can be referenced by VTEP resources via ProviderConfigRef.
	// Provider packages (e.g., cisco/nx) register their GroupVersionKind on init
	// by calling RegisterVTEPDependency, enabling runtime validation of references.
	VTEPDependencies   []schema.GroupVersionKind
	vtepDependenciesMu sync.Mutex
)

// RegisterVTEPDependency adds GVKs to the VTEP dependency registry.This function is typically
// called during package initialization by provider implementations (e.g., VTEPConfig from cisco/nx/v1alpha1)
// to declare themselves as valid ProviderConfigRef targets.
func RegisterVTEPDependency(gvk schema.GroupVersionKind) {
	vtepDependenciesMu.Lock()
	defer vtepDependenciesMu.Unlock()
	VTEPDependencies = append(VTEPDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&VTEP{}, &VTEPList{})
}
