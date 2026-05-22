// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// StaticRouteSpec defines the desired state of StaticRoute
// Static routes are used to define explicit paths for network traffic. They can be categorized into different types based on their characteristics and use cases:
// Directly Connected Routes: Only output interfaces are specified, and the next hop is directly reachable through those interfaces.
// Recursive Static Routes: In a recursive static route, only the next hop is specified. The output interface is derived from the next hop.
// Fully Specified Static Routes: Specifies both the output interfaces and the next hop, providing a complete path for the traffic.
// Floating Static Routes: These routes have a higher administrative distance than dynamic routes, allowing them to serve as backup routes that are only used when the primary route is unavailable.
type StaticRouteSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the static route.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// Description is an optional human-readable description for this static route.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`

	// VrfRef is a reference to the VRF resource that this static route belongs to.
	// If not specified, the static route will be part of the default VRF.
	// The referenced VRF must exist in the same namespace.
	// +optional
	VrfRef *LocalObjectReference `json:"vrfRef,omitempty"`

	// IPPrefix is the destination IP prefix for the static route.
	// +required
	Prefix IPPrefix `json:"prefix"`

	// +required
	// +kubebuilder:validation:MinItems=1
	NextHops []*NextHop `json:"nextHops,omitempty"`
}

type NextHop struct {
	// TODO(sven-rosenzweig): It is possible to point an a static route in a VRF to an Interface. For now this is not needed.
	// InterfaceRef is a reference to the Interface resource that this static route is associated with.
	// The referenced Interface must exist in the same namespace.
	// +optional
	// InterfaceRef *LocalObjectReference `json:"interfaceRef,omitempty"`

	// Address is the IP address of the next hop for the static route.
	// +required
	Address string `json:"address,omitempty"`

	// Metric assigns a priority to the static route. Lower values indicate higher priority.
	// +optional
	Metric *int32 `json:"metric,omitempty"`
}

// StaticRouteStatus defines the observed state of StaticRoute.
type StaticRouteStatus struct {
	// The conditions are a list of status objects that describe the state of the StaticRoute.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="VRF",type=string,JSONPath=`.spec.vrfRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// StaticRoute is the Schema for the staticroutes API
type StaticRoute struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of StaticRoute
	// +required
	Spec StaticRouteSpec `json:"spec"`

	// status defines the observed state of StaticRoute
	// +optional
	Status StaticRouteStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (sr *StaticRoute) GetConditions() []metav1.Condition {
	return sr.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (sr *StaticRoute) SetConditions(conditions []metav1.Condition) {
	sr.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// StaticRouteList contains a list of StaticRoute
type StaticRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []StaticRoute `json:"items"`
}

var (
	StaticRouteDependencies   []schema.GroupVersionKind
	staticRouteDependenciesMu sync.Mutex
)

func RegisterStaticRouteDependency(gvk schema.GroupVersionKind) {
	staticRouteDependenciesMu.Lock()
	defer staticRouteDependenciesMu.Unlock()
	StaticRouteDependencies = append(StaticRouteDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &StaticRoute{}, &StaticRouteList{})
		return nil
	})
}
