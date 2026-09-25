// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CommunityMember defines one ordered entry in a community-list with a regex pattern.
type CommunityMember struct {
	// Sequence is the order of this entry in the community-list.
	// +required
	// +kubebuilder:validation:Minimum=1
	Sequence int32 `json:"sequence"`

	// Regex is a POSIX extended regular expression matching BGP community values.
	// +required
	// +kubebuilder:validation:MinLength=1
	Regex string `json:"regex"`
}

// CommunitySetType selects whether the CommunitySet is a standard BGP
// community-list or an extended community-list.
// +kubebuilder:validation:Enum=standard;extended
type CommunitySetType string

const (
	// CommunitySetTypeStandard is a standard BGP community-list.
	CommunitySetTypeStandard CommunitySetType = "standard"
	// CommunitySetTypeExtended is an extended BGP community-list.
	CommunitySetTypeExtended CommunitySetType = "extended"
)

// CommunitySetSpec defines the desired state of CommunitySet.
type CommunitySetSpec struct {
	// DeviceRef is a reference to the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the CommunitySet on the device.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// Type is the type of the CommunitySet. It can be either "standard" (a
	// standard BGP community-list) or "extended" (an extended community-list).
	// Immutable.
	// +required
	// +kubebuilder:default=standard
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	Type CommunitySetType `json:"type"`

	// Members is the ordered list of community-list entries.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Members []CommunityMember `json:"members"`
}

// CommunitySetStatus defines the observed state of CommunitySet.
type CommunitySetStatus struct {
	// Conditions is a list of status conditions describing the state of the CommunitySet.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=communitysets
// +kubebuilder:resource:singular=communityset
// +kubebuilder:printcolumn:name="Community Set",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CommunitySet is the Schema for the communitysets API.
type CommunitySet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec CommunitySetSpec `json:"spec,omitempty"`

	// +optional
	Status CommunitySetStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (c *CommunitySet) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (c *CommunitySet) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// CommunitySetList contains a list of CommunitySet.
type CommunitySetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CommunitySet `json:"items"`
}

var (
	CommunitySetDependencies   []schema.GroupVersionKind
	communitySetDependenciesMu sync.Mutex
)

func RegisterCommunitySetDependency(gvk schema.GroupVersionKind) {
	communitySetDependenciesMu.Lock()
	defer communitySetDependenciesMu.Unlock()
	CommunitySetDependencies = append(CommunitySetDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &CommunitySet{}, &CommunitySetList{})
		return nil
	})
}
