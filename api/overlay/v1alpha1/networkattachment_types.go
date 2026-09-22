// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// NetworkAttachmentSpec defines the desired state of NetworkAttachment
type NetworkAttachmentSpec struct {
	// NetworkRef references the [Network] this attachment is associated with.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="networkRef is immutable"
	NetworkRef corev1alpha1.LocalObjectReference `json:"networkRef"`

	// InterfaceSelector selects Interface resources to attach to the network.
	// Type filtering (Physical ∪ Aggregate) is enforced by the controller at
	// reconcile time, not by the selector itself.
	// +required
	InterfaceSelector metav1.LabelSelector `json:"interfaceSelector"`

	// Encapsulation defines the encapsulation parameters for this attachment.
	// +required
	Encapsulation Encapsulation `json:"encapsulation"`
}

// EncapsulationType is the encapsulation method used for a NetworkAttachment.
// +kubebuilder:validation:Enum=VLAN
type EncapsulationType string

const (
	// EncapsulationTypeVLAN uses IEEE 802.1Q VLAN tagging.
	EncapsulationTypeVLAN EncapsulationType = "VLAN"
)

// Encapsulation defines the encapsulation parameters for a NetworkAttachment.
type Encapsulation struct {
	// Type is the encapsulation method.
	// +required
	Type EncapsulationType `json:"type"`

	// Id is the identifier for the encapsulation method. For VLAN, this is the VLAN ID.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	ID int32 `json:"id"`
}

// NetworkAttachmentStatus defines the observed state of NetworkAttachment.
type NetworkAttachmentStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the NetworkAttachment resource.
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
// +kubebuilder:resource:path=networkattachments
// +kubebuilder:resource:singular=networkattachment
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.spec.networkRef.name`
// +kubebuilder:printcolumn:name="Encapsulation",type=string,JSONPath=`.spec.encapsulation.type`
// +kubebuilder:printcolumn:name="VLAN",type=integer,JSONPath=`.spec.encapsulation.id`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NetworkAttachment is the Schema for the networkattachments API
type NetworkAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec NetworkAttachmentSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status NetworkAttachmentStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (n *NetworkAttachment) GetConditions() []metav1.Condition {
	return n.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (n *NetworkAttachment) SetConditions(conds []metav1.Condition) {
	n.Status.Conditions = conds
}

// +kubebuilder:object:root=true

// NetworkAttachmentList contains a list of NetworkAttachment
type NetworkAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NetworkAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &NetworkAttachment{}, &NetworkAttachmentList{})
		return nil
	})
}
