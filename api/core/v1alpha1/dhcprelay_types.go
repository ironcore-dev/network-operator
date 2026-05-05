// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DHCPRelaySpec defines the desired state of DHCPRelay.
// Only a single DHCPRelay resource should be created per Device, the controller will reject additional resources of this type with the same DeviceRef.
type DHCPRelaySpec struct {
	// DeviceRef is a reference to the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration for this DHCPRelay.
	// If not specified the provider applies the target platform's default settings.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// VrfRef is an optional reference to the VRF to use when relaying DHCP messages in all referenced interfaces.
	// +optional
	VrfRef *LocalObjectReference `json:"vrfRef,omitempty"`

	// Servers is a list of DHCP server addresses to which DHCP messages will be relayed.
	// Only IPv4 addresses are currently supported.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:items:Format=ipv4
	// +kubebuilder:validation:MinItems=1
	Servers []string `json:"servers"`

	// InterfaceRefs is a list of interfaces
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	InterfaceRefs []LocalObjectReference `json:"interfaceRefs,omitempty"`
}

// DHCPRelayStatus defines the observed state of DHCPRelay.
type DHCPRelayStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the DHCPRelay resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConfiguredInterfaces contains the names of Interface resources that have DHCP relay configured as known by the device.
	// +optional
	// +listType=atomic
	ConfiguredInterfaces []string `json:"configuredInterfaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dhcprelays
// +kubebuilder:resource:singular=dhcprelay
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DHCPRelay is the Schema for the DHCPRelays API
type DHCPRelay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec DHCPRelaySpec `json:"spec"`

	// +optional
	Status DHCPRelayStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (l *DHCPRelay) GetConditions() []metav1.Condition {
	return l.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (l *DHCPRelay) SetConditions(conditions []metav1.Condition) {
	l.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// DHCPRelayList contains a list of DHCPRelay
type DHCPRelayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DHCPRelay `json:"items"`
}

var (
	DHCPRelayDependencies   []schema.GroupVersionKind
	DHCPRelayDependenciesMu sync.Mutex
)

// RegisterDHCPRelayDependency registers a provider-specific GVK as a dependency of DHCPRelay.
// ProviderConfigs should call this in their init() function to ensure the dependency is registered.
func RegisterDHCPRelayDependency(gvk schema.GroupVersionKind) {
	DHCPRelayDependenciesMu.Lock()
	defer DHCPRelayDependenciesMu.Unlock()
	DHCPRelayDependencies = append(DHCPRelayDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &DHCPRelay{}, &DHCPRelayList{})
		return nil
	})
}
