// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// EVPNInterconnectSpec defines the desired state of EVPNInterconnect
type EVPNInterconnectSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// members represents the list of members in the EVPN interconnect.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=2
	// TODO: admission webhook  and reconciliation error to validate we have exactly two different devices
	Interfaces []DCIInterface `json:"interfaces"`

	// +required
	DCI DCI `json:"dci"`

	// multiSiteIDPool represents the pool of Multi-Site IDs (VNIs) for the EVPN interconnect.
	// +required
	MultiSiteIDIndexPoolRef corev1alpha1.TypedLocalObjectReference `json:"multiSiteIDIndexPoolRef"`

	// ipPrefixPoolRef references the IPPrefixPool from which IP addresses for DCI interfaces are allocated.
	// +required
	IPAddressPoolRef corev1alpha1.LocalObjectReference `json:"ipAddressPoolRef"`
}

// Member defines a member of the EVPN interconnect.
type DCIInterface struct {
	// interfaces is a list of interfaces facing DCI
	// +required
	InterfaceRef corev1alpha1.LocalObjectReference `json:"interfaceRef"`

	// ipAddress is the IP address of the interface facing DCI
	// +required
	Address corev1alpha1.IPPrefix `json:"address"`

	// remoteAddress is the IP address of the remote interface facing DCI
	// +required
	RemoteAddress corev1alpha1.IPPrefix `json:"remoteAddress"`

	// mtu is the MTU of the interface facing DCI
	// +optional
	// +kubebuilder:validation:Minimum=576
	// +kubebuilder:validation:Maximum=9216
	MTU *int32 `json:"mtu,omitempty"`
}

type DCI struct {
	// +required
	Underlay Underlay `json:"underlay"`

	// +required
	Overlay Overlay `json:"overlay"`
}

// +kubebuilder:validation:Enum=OSPF;BGP
type UnderlayProtocol string

const (
	UnderlayProtocolOSPF UnderlayProtocol = "OSPF"
	UnderlayProtocolBGP  UnderlayProtocol = "BGP"
)

// Underlay represents the underlay network configuration for the DCI.
type Underlay struct {
	// protocol represents the underlay routing protocol used for the DCI underlay.
	// +required
	Protocol UnderlayProtocol `json:"protocol"`

	// +optional
	OSPF OSPF `json:"ospf,omitempty"`

	// +optional
	BGP BGP `json:"bgp,omitempty"`
}

type OSPF struct{}

type BGP struct{}

// +kubebuilder:validation:Enum=BGP
type OverlayProtocol string

const (
	OverlayProtocolBGP OverlayProtocol = "BGP"
)

// Overlay represents the overlay network configuration for the DCI.
type Overlay struct {
	// protocol represents the overlay routing protocol used for the DCI overlay.
	// +required
	Protocol OverlayProtocol `json:"protocol"`

	// +required
	EVPNInterconnectSelector metav1.LabelSelector `json:"evpnInterconnectSelector"`

	// +optional
	BGP OverlayBGP `json:"bgp,omitempty"`
}

type OverlayBGP struct {
	MultiHop *int32 `json:"multiHop,omitempty"`
}

// EVPNInterconnectStatus defines the observed state of EVPNInterconnect.
type EVPNInterconnectStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the EVPNInterconnect resource.
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

// GetConditions implements conditions.Getter.
func (e *EVPNInterconnect) GetConditions() []metav1.Condition {
	return e.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (e *EVPNInterconnect) SetConditions(conditions []metav1.Condition) {
	e.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// EVPNInterconnect is the Schema for the evpninterconnects API
type EVPNInterconnect struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of EVPNInterconnect
	// +required
	Spec EVPNInterconnectSpec `json:"spec"`

	// status defines the observed state of EVPNInterconnect
	// +optional
	Status EVPNInterconnectStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EVPNInterconnectList contains a list of EVPNInterconnect
type EVPNInterconnectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []EVPNInterconnect `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &EVPNInterconnect{}, &EVPNInterconnectList{})
		return nil
	})
}
