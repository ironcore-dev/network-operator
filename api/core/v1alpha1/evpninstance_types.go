// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EVPNInstanceSpec defines the desired state of EVPNInstance
//
// It models an EVPN instance (EVI) context on a single network device based on VXLAN encapsulation and the VLAN-based service type defined in [RFC 8365].
// [RFC 8365]: https://datatracker.ietf.org/doc/html/rfc8365
//
// +kubebuilder:validation:XValidation:rule="self.type != 'Bridged' || has(self.vlanRef)",message="VLANRef must be specified when Type is Bridged"
type EVPNInstanceSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the BGP to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// VNI is the VXLAN Network Identifier.
	// Immutable.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16777214
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="VNI is immutable"
	VNI int32 `json:"vni"`

	// Type specifies the EVPN instance type.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	Type EVPNInstanceType `json:"type"`

	// MulticastGroupAddress specifies the IPv4 multicast group address used for BUM (Broadcast, Unknown unicast, Multicast) traffic.
	// The address must be in the valid multicast range (224.0.0.0 - 239.255.255.255).
	// +optional
	// +kubebuilder:validation:Format=ipv4
	MulticastGroupAddress string `json:"multicastGroupAddress,omitempty"`

	// RouteDistinguisher is the route distinguisher for the EVI.
	// Formats supported:
	//  - Type 0: <asn(0-65535)>:<number(0-4294967295)>
	//  - Type 1: <ipv4>:<number(0-65535)>
	//  - Type 2: <asn(65536-4294967295)>:<number(0-65535)>
	// +optional
	RouteDistinguisher string `json:"routeDistinguisher,omitempty"`

	// RouteTargets is the list of route targets for the EVI.
	// +optional
	// +listType=map
	// +listMapKey=value
	// +kubebuilder:validation:MinItems=1
	RouteTargets []EVPNRouteTarget `json:"routeTargets,omitempty"`

	// VLANRef is a reference to a VLAN resource for which this EVPNInstance builds the MAC-VRF.
	// This field is only applicable when Type is Bridged (L2VNI).
	// The VLAN resource must exist in the same namespace.
	// Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="VLANRef is immutable"
	VLANRef *LocalObjectReference `json:"vlanRef,omitempty"`
}

// EVPNInstanceType defines the type of EVPN instance.
// +kubebuilder:validation:Enum=Bridged;Routed
type EVPNInstanceType string

const (
	// EVPNInstanceTypeBridged represents an L2VNI (MAC-VRF) EVPN instance.
	// Corresponds to OpenConfig network-instance type L2VSI.
	EVPNInstanceTypeBridged EVPNInstanceType = "Bridged"

	// EVPNInstanceTypeRouted represents an L3VNI (IP-VRF) EVPN instance.
	// Corresponds to OpenConfig network-instance type L3VRF.
	EVPNInstanceTypeRouted EVPNInstanceType = "Routed"
)

type EVPNRouteTarget struct {
	// Value is the route target value, must have the format as RouteDistinguisher.
	// +required
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value"`

	// Action defines whether the route target is imported, exported, or both.
	// +required
	Action RouteTargetAction `json:"action"`
}

// EVPNInstanceStatus defines the observed state of EVPNInstance.
type EVPNInstanceStatus struct {
	// The conditions are a list of status objects that describe the state of the EVPNInstance.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=evpninstances
// +kubebuilder:resource:singular=evpninstance
// +kubebuilder:resource:shortName=evi;vni
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="VNI",type=integer,JSONPath=`.spec.vni`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EVPNInstance is the Schema for the evpninstances API
type EVPNInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec EVPNInstanceSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status EVPNInstanceStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (i *EVPNInstance) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (i *EVPNInstance) SetConditions(conditions []metav1.Condition) {
	i.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// EVPNInstanceList contains a list of EVPNInstance
type EVPNInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EVPNInstance `json:"items"`
}

var (
	EVPNInstanceDependencies   []schema.GroupVersionKind
	evpnInstanceDependenciesMu sync.Mutex
)

func RegisterEVPNInstanceDependency(gvk schema.GroupVersionKind) {
	evpnInstanceDependenciesMu.Lock()
	defer evpnInstanceDependenciesMu.Unlock()
	EVPNInstanceDependencies = append(EVPNInstanceDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&EVPNInstance{}, &EVPNInstanceList{})
}
