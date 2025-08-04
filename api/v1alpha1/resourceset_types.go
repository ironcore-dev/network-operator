// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ResourceSetSpec defines the desired state of ResourceSet.
type ResourceSetSpec struct {
	// Mode defines how the resources in the ResourceSet are applied and managed.
	// +kubebuilder:default=Reconcile
	Mode ResourceSetMode `json:"mode,omitempty"`

	// Selector is a label query over a set of resources.
	// +required
	Selector metav1.LabelSelector `json:"selector"`

	// Resources is a list of resources that are part of this ResourceSet.
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	// +required
	Resources []Resource `json:"resources,omitempty"`
}

// ResourceSetMode determines how the resources in a ResourceSet are applied and managed.
// +kubebuilder:validation:Enum=ApplyOnce;Reconcile
type ResourceSetMode string

const (
	// ResourceSetModeApplyOnce means the resources are applied once and not managed afterwards.
	ResourceSetModeApplyOnce = "ApplyOnce"
	// ResourceSetModeReconcile means the resources are continuously reconciled to match the desired spec.
	ResourceSetModeReconcile = "Reconcile"
)

// Resource defines a resource that is part of a ResourceSet.
type Resource struct {
	// Name is a user-defined, unique name for this template within the ResourceSet.
	// This name is used to generate the final name of the created resource.
	// It must be unique within the `resources` list of the [ResourceSetSpec].
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
	// Kind is the kind of the resource.
	// +required
	Kind string `json:"kind"`
	// APIVersion is the API version of the resource.
	// +required
	APIVersion string `json:"apiVersion"`
	// Template is the template for the resource.
	// It is a raw extension that can contain any JSON object.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +required
	Template runtime.RawExtension `json:"template"`
}

// ResourceSetStatus defines the observed state of ResourceSet.
type ResourceSetStatus struct {
	// ManagedResources is a list of all resources created and managed by this ResourceSet.
	// +optional
	ManagedResources []ManagedResource `json:"managedResources,omitempty"`

	// The conditions are a list of status objects that describe the state of the ResourceSet.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ManagedResource struct {
	// Name of the managed resource.
	// +required
	Name string `json:"name"`
	// Kind of the managed resource.
	// +required
	Kind string `json:"kind"`
	// APIVersion of the managed resource.
	// +required
	APIVersion string `json:"apiVersion"`
	// Namespace of the managed resource.
	// +required
	Namespace string `json:"namespace"`
	// TargetName is the name of the device resource that this child was created for.
	// +required
	TargetName string `json:"targetName"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=resourcesets
// +kubebuilder:resource:singular=resourceset
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResourceSet is the Schema for the resourcesets API.
type ResourceSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSetSpec   `json:"spec,omitempty"`
	Status ResourceSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceSetList contains a list of ResourceSet.
type ResourceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceSet{}, &ResourceSetList{})
}
