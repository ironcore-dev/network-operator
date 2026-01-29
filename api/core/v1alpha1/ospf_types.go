// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// OSPFSpec defines the desired state of OSPF
type OSPFSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// AdminState indicates whether the OSPF instance is administratively up or down.
	// +optional
	// +kubebuilder:default=Up
	AdminState AdminState `json:"adminState,omitempty"`

	// Instance is the process tag of the OSPF instance.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Instance is immutable"
	Instance string `json:"instance"`

	// RouterID is the OSPF router identifier, used in OSPF messages to identify the originating router.
	// Follows dotted quad notation (IPv4 format).
	// +required
	// +kubebuilder:validation:Format=ipv4
	RouterID string `json:"routerId"`

	// LogAdjacencyChanges enables logging when the state of an OSPF neighbor changes.
	// When true, a log message is generated for adjacency state transitions.
	// +optional
	LogAdjacencyChanges *bool `json:"logAdjacencyChanges,omitempty"`

	// InterfaceRefs is a list of interfaces that are part of the OSPF instance.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	InterfaceRefs []OSPFInterface `json:"interfaceRefs,omitempty"`
}

// OSPFInterface defines the OSPF-specific configuration for an interface
// that is participating in an OSPF instance.
type OSPFInterface struct {
	LocalObjectReference `json:",inline"`

	// Area is the OSPF area identifier for this interface.
	// Must be specified in dotted-quad notation (e.g., "0.0.0.0" for the backbone area).
	// This is semantically a 32-bit identifier displayed in IPv4 address format,
	// not an actual IPv4 address. Area 0 (0.0.0.0) is the OSPF backbone area and
	// is required for proper OSPF operation in multi-area configurations.
	// +required
	// +kubebuilder:validation:Format=ipv4
	Area string `json:"area"`

	// Passive indicates whether this interface should operate in passive mode.
	// In passive mode, OSPF will advertise the interface's network in LSAs but will not
	// send or receive OSPF protocol packets (Hello, LSU, etc.) on this interface.
	// This is typically used for loopback interfaces where OSPF adjacencies
	// should not be formed but the network should still be advertised.
	// Defaults to false (active mode).
	// +optional
	Passive *bool `json:"passive,omitempty"`
}

// OSPFStatus defines the observed state of OSPF.
type OSPFStatus struct {
	// AdjacencySummary provides a human-readable summary of neighbor adjacencies
	// by state (e.g., "3 Full, 1 ExStart, 1 Down").
	// This field is computed by the controller from the Neighbors field.
	// +optional
	AdjacencySummary string `json:"adjacencySummary,omitempty"`

	// ObservedGeneration reflects the .metadata.generation that was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Neighbors is a list of OSPF neighbors and their adjacency states.
	// +optional
	// +listType=map
	// +listMapKey=routerId
	// +patchStrategy=merge
	// +patchMergeKey=routerId
	Neighbors []OSPFNeighbor `json:"neighbors,omitempty"`

	// The conditions are a list of status objects that describe the state of the OSPF.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OSPFNeighbor represents an OSPF neighbor with its adjacency information.
type OSPFNeighbor struct {
	// RouterID is the router identifier of the remote OSPF neighbor.
	// +required
	RouterID string `json:"routerId"`

	// Address is the IP address of the remote OSPF neighbor.
	// +required
	Address string `json:"address"`

	// InterfaceRef is a reference to the local interface through which this neighbor is connected.
	// +required
	InterfaceRef LocalObjectReference `json:"interfaceRef"`

	// Priority is the remote system's priority to become the designated router.
	// Valid range is 0-255.
	// +optional
	Priority *uint8 `json:"priority,omitempty"`

	// LastEstablishedTime is the timestamp when the adjacency last transitioned to the FULL state.
	// A frequently changing timestamp indicates adjacency instability (flapping).
	// +optional
	LastEstablishedTime *metav1.Time `json:"lastEstablishedTime,omitempty"`

	// AdjacencyState is the current state of the adjacency with this neighbor.
	// +optional
	AdjacencyState OSPFNeighborState `json:"adjacencyState,omitempty"`
}

// OSPFNeighborState represents the state of an OSPF adjacency as defined in RFC 2328.
// +kubebuilder:validation:Enum=Down;Attempt;Init;TwoWay;ExStart;Exchange;Loading;Full
type OSPFNeighborState string

const (
	// OSPFNeighborStateUnknown indicates an unknown or undefined state.
	OSPFNeighborStateUnknown OSPFNeighborState = "Unknown"

	// OSPFNeighborStateDown indicates the initial state of a neighbor.
	// No recent information has been received from the neighbor.
	OSPFNeighborStateDown OSPFNeighborState = "Down"

	// OSPFNeighborStateAttempt is only valid for neighbors on NBMA networks.
	// It indicates that no recent information has been received but effort should be made to contact the neighbor.
	OSPFNeighborStateAttempt OSPFNeighborState = "Attempt"

	// OSPFNeighborStateInit indicates a Hello packet has been received from the neighbor
	// but bidirectional communication has not yet been established.
	OSPFNeighborStateInit OSPFNeighborState = "Init"

	// OSPFNeighborStateTwoWay indicates bidirectional communication has been established.
	// This is the most advanced state short of forming an adjacency.
	OSPFNeighborStateTwoWay OSPFNeighborState = "TwoWay"

	// OSPFNeighborStateExStart indicates the first step in creating an adjacency.
	// The routers are determining the relationship and initial DD sequence number.
	OSPFNeighborStateExStart OSPFNeighborState = "ExStart"

	// OSPFNeighborStateExchange indicates the routers are exchanging Database Description packets.
	OSPFNeighborStateExchange OSPFNeighborState = "Exchange"

	// OSPFNeighborStateLoading indicates Link State Request packets are being sent to the neighbor
	// to obtain more recent LSAs that were discovered during the Exchange state.
	OSPFNeighborStateLoading OSPFNeighborState = "Loading"

	// OSPFNeighborStateFull indicates the neighboring routers are fully adjacent.
	// LSDBs are synchronized and the adjacency will appear in Router and Network LSAs.
	OSPFNeighborStateFull OSPFNeighborState = "Full"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ospf
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instance`
// +kubebuilder:printcolumn:name="Router-ID",type=string,JSONPath=`.spec.routerId`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Adjacencies",type=string,JSONPath=`.status.adjacencySummary`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OSPF is the Schema for the ospf API
type OSPF struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec OSPFSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status OSPFStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (o *OSPF) GetConditions() []metav1.Condition {
	return o.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (o *OSPF) SetConditions(conditions []metav1.Condition) {
	o.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// OSPFList contains a list of OSPF
type OSPFList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OSPF `json:"items"`
}

var (
	OSPFDependencies   []schema.GroupVersionKind
	ospfDependenciesMu sync.Mutex
)

func RegisterOSPFDependency(gvk schema.GroupVersionKind) {
	ospfDependenciesMu.Lock()
	defer ospfDependenciesMu.Unlock()
	OSPFDependencies = append(OSPFDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&OSPF{}, &OSPFList{})
}
