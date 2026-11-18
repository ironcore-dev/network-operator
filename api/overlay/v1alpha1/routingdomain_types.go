// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RoutingDomainSpec defines the desired state of RoutingDomain
type RoutingDomainSpec struct {
	// name of the VRF created for the RoutingDomain.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`
}

// RoutingDomainStatus defines the observed state of RoutingDomain.
type RoutingDomainStatus struct {
	// conditions represent the current state of the RoutingDomain resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Routing Policy",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RoutingDomain is the Schema for the routingdomains API
type RoutingDomain struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RoutingDomain
	// +required
	Spec RoutingDomainSpec `json:"spec"`

	// status defines the observed state of RoutingDomain
	// +optional
	Status RoutingDomainStatus `json:"status,omitzero"`
}

// GetConditions returns the list of conditions for the RoutingDomain.
func (rd *RoutingDomain) GetConditions() []metav1.Condition {
	return rd.Status.Conditions
}

// SetConditions sets the list of conditions for the RoutingDomain.
func (rd *RoutingDomain) SetConditions(conditions []metav1.Condition) {
	rd.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// RoutingDomainList contains a list of RoutingDomain
type RoutingDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RoutingDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &RoutingDomain{}, &RoutingDomainList{})
		return nil
	})
}
