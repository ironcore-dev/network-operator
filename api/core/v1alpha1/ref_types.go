// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

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
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?([a-z0-9]([-a-z0-9]*[a-z0-9])?)$`
	APIVersion string `json:"apiVersion"`
}

// TypedLocalObjectRefFromObject builds a TypedLocalObjectReference from a [client.Object].
func TypedLocalObjectRefFromObject(obj client.Object) *TypedLocalObjectReference {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return &TypedLocalObjectReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       obj.GetName(),
	}
}

// SecretReference represents a Secret Reference. It has enough information to retrieve a Secret
// in any namespace.
// +structType=atomic
type SecretReference struct {
	// Name is unique within a namespace to reference a secret resource.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Namespace defines the space within which the secret name must be unique.
	// If omitted, the namespace of the object being reconciled will be used.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace,omitempty"`
}

// SecretKeySelector contains enough information to select a key of a Secret.
// +structType=atomic
type SecretKeySelector struct {
	// Name of the secret resource being referred to.
	SecretReference `json:",inline"`

	// Key is the of the entry in the secret resource's `data` or `stringData`
	// field to be used.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key"`
}

// ConfigMapReference represents a ConfigMap Reference. It has enough information to retrieve a ConfigMap
// in any namespace.
// +structType=atomic
type ConfigMapReference struct {
	// Name is unique within a namespace to reference a configmap resource.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Namespace defines the space within which the configmap name must be unique.
	// If omitted, the namespace of the object being reconciled will be used.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace,omitempty"`
}

// ConfigMapKeySelector contains enough information to select a key of a ConfigMap.
// +structType=atomic
type ConfigMapKeySelector struct {
	// Name of the configmap resource being referred to.
	ConfigMapReference `json:",inline"`

	// Key is the of the entry in the configmap resource's `data` or `binaryData`
	// field to be used.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key"`
}

// InterfaceSource identifies a interface either by literal name or by reference to a managed Interface resource.
// Exactly one of Name or InterfaceRef must be specified.
// +kubebuilder:validation:XValidation:rule="(has(self.name) && !has(self.interfaceRef)) || (!has(self.name) && has(self.interfaceRef))",message="exactly one of name or interfaceRef must be specified"
type InterfaceSource struct {
	// Name is the literal interface name on the device (e.g., "mgmt0", "Loopback0").
	// Use this for interfaces that are not managed as Interface resources.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name,omitempty"`

	// InterfaceRef references a managed Interface resource in the same namespace.
	// The controller resolves the device interface name from this resource.
	// +optional
	InterfaceRef *LocalObjectReference `json:"interfaceRef,omitempty"`
}

// VLANSource identifies a VLAN either by literal ID or by reference to a managed VLAN resource.
// Exactly one of ID or VLANRef must be specified.
// +kubebuilder:validation:XValidation:rule="(has(self.id) && !has(self.vlanRef)) || (!has(self.id) && has(self.vlanRef))",message="exactly one of id or vlanRef must be specified"
type VLANSource struct {
	// ID is the literal VLAN ID on the device (1-4094).
	// Use this for VLANs that are not managed as VLAN resources.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	ID *int16 `json:"id,omitempty"`

	// VLANRef references a managed VLAN resource in the same namespace.
	// The controller resolves the VLAN ID from this resource.
	// +optional
	VLANRef *LocalObjectReference `json:"vlanRef,omitempty"`
}

// VRFSource identifies a VRF/NetworkIntance either by literal name or by reference to a managed VRF resource.
// Exactly one of Name or VRFRef must be specified.
// +kubebuilder:validation:XValidation:rule="(has(self.name) && !has(self.vrfRef)) || (!has(self.name) && has(self.vrfRef))",message="exactly one of name or vrfRef must be specified"
type VRFSource struct {
	// Name is the literal VRF name on the device (e.g., "management", "default").
	// Use this for VRFs that are not managed as VRF resources.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name,omitempty"`

	// VRFRef references a managed VRF resource in the same namespace.
	// The controller resolves the device VRF name from this resource.
	// +optional
	VRFRef *LocalObjectReference `json:"vrfRef,omitempty"`
}
