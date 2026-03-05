// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// LLDPSpec defines the desired state of LLDP
type LLDPSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration for this LLDP.
	// If not specified the provider applies the target platform's default settings.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether LLDP is system-wide administratively up or down.
	// +required
	AdminState AdminState `json:"adminState"`

	// InterfaceRefs is a list of interfaces and their LLDP configuration.
	// +optional
	// +listType=atomic
	InterfaceRefs []LLDPInterface `json:"interfaceRefs,omitempty"`
}

type LLDPInterface struct {
	LocalObjectReference `json:",inline"`

	// AdminState indicates whether LLDP is administratively up or down on this interface.
	// This will be ignored if LLDP is configured to be administratively down system-wide.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState"`
}

// LLDPStatus defines the observed state of LLDP.
type LLDPStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the LLDP resource.
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=lldps
// +kubebuilder:resource:singular=lldp
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LLDP is the Schema for the lldps API
type LLDP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec LLDPSpec `json:"spec"`

	// +optional
	Status LLDPStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (l *LLDP) GetConditions() []metav1.Condition {
	return l.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (l *LLDP) SetConditions(conditions []metav1.Condition) {
	l.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// LLDPList contains a list of LLDP
type LLDPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LLDP `json:"items"`
}

var (
	LLDPDependencies   []schema.GroupVersionKind
	lldpDependenciesMu sync.Mutex
)

// RegisterLLDPDependency registers a provider-specific GVK as a dependency of LLDP.
// ProviderConfigs should call this in their init() function to ensure the dependency is registered.
func RegisterLLDPDependency(gvk schema.GroupVersionKind) {
	lldpDependenciesMu.Lock()
	defer lldpDependenciesMu.Unlock()
	LLDPDependencies = append(LLDPDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&LLDP{}, &LLDPList{})
}
