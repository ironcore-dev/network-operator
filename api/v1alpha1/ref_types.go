// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

// LocalObjectReference contains enough information to locate a
// referenced object inside the same namespace.
// +structType=atomic
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`
}

// TypedLocalObjectReference contains enough information to locate a
// typed referenced object inside the same namespace.
// +structType=atomic
type TypedLocalObjectReference struct {
	// Kind of the resource being referenced.
	// Kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`
	Kind string `json:"kind"`

	// Name of the resource being referenced.
	// Name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`

	// APIVersion is the api group version of the resource being referenced.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	//+kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?([a-z0-9]([-a-z0-9]*[a-z0-9])?)$`
	APIVersion string `json:"apiVersion"`
}
