// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EthernetSegmentSpec defines the desired state of EthernetSegment.
//
// It models an EVPN Ethernet Segment for multihoming as defined in [RFC 7432] Section 5.
// An Ethernet Segment associates an Aggregate interface with a 10-byte Ethernet Segment
// Identifier (ESI), enabling multi-homed CE connectivity.
// [RFC 7432]: https://datatracker.ietf.org/doc/html/rfc7432
//
// +kubebuilder:validation:XValidation:rule="self.esiType != 'Arbitrary' || has(self.esi)",message="ESI is required when ESIType is Arbitrary"
// +kubebuilder:validation:XValidation:rule="!(self.esiType == 'LACP' || self.esiType == 'MST') || !has(self.esi)",message="ESI must be omitted when ESIType is LACP or MST"
// +kubebuilder:validation:XValidation:rule="!has(self.esi) || (self.esiType == 'Arbitrary' && self.esi.startsWith('00')) || (self.esiType == 'MAC' && self.esi.startsWith('03')) || (self.esiType == 'RouterID' && self.esi.startsWith('04')) || (self.esiType == 'AS' && self.esi.startsWith('05'))",message="ESI first byte must match ESIType"
type EthernetSegmentSpec struct {
	// DeviceRef is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this Ethernet Segment.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// InterfaceRef is the name of the Interface this Ethernet Segment is associated with.
	// The Interface must be of type Aggregate and belong to the same Device.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="InterfaceRef is immutable"
	InterfaceRef LocalObjectReference `json:"interfaceRef"`

	// ESIType selects the ESI derivation method (RFC 7432 Section 5).
	// When Arbitrary (Type 0), ESI must be provided explicitly.
	// When LACP or MST (Types 1, 2), ESI is always auto-derived (ESI field must be omitted).
	// When MAC, RouterID, or AS (Types 3-5), ESI may be explicit or auto-derived.
	// +required
	// +kubebuilder:default=Arbitrary
	ESIType ESIType `json:"esiType"`

	// ESI is the 10-byte Ethernet Segment Identifier in colon-separated hex notation
	// (e.g., "00:11:22:33:44:55:66:77:88:01"). Must not be all-zeros or all-ones (reserved per RFC 7432).
	// Required when ESIType is Arbitrary. Must be omitted when ESIType is LACP or MST.
	// Optional for MAC, RouterID, and AS types (omit to auto-derive on the device).
	// +optional
	// +kubebuilder:validation:Pattern=`^([0-9a-fA-F]{2}:){9}[0-9a-fA-F]{2}$`
	// +kubebuilder:validation:XValidation:rule="self.lowerAscii() != '00:00:00:00:00:00:00:00:00:00'",message="ESI must not be all-zeros (reserved for single-homed per RFC 7432)"
	// +kubebuilder:validation:XValidation:rule="self.lowerAscii() != 'ff:ff:ff:ff:ff:ff:ff:ff:ff:ff'",message="ESI must not be MAX-ESI (reserved per RFC 7432)"
	ESI string `json:"esi,omitempty"`

	// RedundancyMode defines the multi-homing forwarding model for this Ethernet Segment
	// as defined in RFC 7432 Section 14.1.
	// +kubebuilder:validation:Enum=AllActive;SingleActive
	// +kubebuilder:default=AllActive
	// +optional
	RedundancyMode RedundancyMode `json:"redundancyMode,omitempty"`

	// DesignatedForwarder configures the Designated Forwarder election for this
	// Ethernet Segment (RFC 7432 Section 8.5, RFC 8584).
	// +optional
	DesignatedForwarder *DesignatedForwarder `json:"designatedForwarder,omitempty"`
}

// ESIType defines the ESI derivation method per RFC 7432 Section 5.
// +kubebuilder:validation:Enum=Arbitrary;LACP;MST;MAC;RouterID;AS
type ESIType string

const (
	// ESITypeArbitrary indicates an operator-configured ESI value (Type 0).
	ESITypeArbitrary ESIType = "Arbitrary"

	// ESITypeLACP indicates a LACP-based ESI derived from CE system MAC and port key (Type 1).
	ESITypeLACP ESIType = "LACP"

	// ESITypeMST indicates a bridge-protocol-based ESI derived from root bridge parameters (Type 2).
	ESITypeMST ESIType = "MST"

	// ESITypeMAC indicates a MAC-based ESI derived from system MAC and local discriminator (Type 3).
	ESITypeMAC ESIType = "MAC"

	// ESITypeRouterID indicates a router-ID-based ESI (Type 4).
	ESITypeRouterID ESIType = "RouterID"

	// ESITypeAS indicates an AS-number-based ESI (Type 5).
	ESITypeAS ESIType = "AS"
)

// RedundancyMode defines the forwarding model for a multi-homed Ethernet Segment.
// +kubebuilder:validation:Enum=AllActive;SingleActive
type RedundancyMode string

const (
	// RedundancyModeAllActive enables all PE nodes in the segment to forward unicast
	// traffic simultaneously (RFC 7432 Section 14.1.2).
	RedundancyModeAllActive RedundancyMode = "AllActive"

	// RedundancyModeSingleActive restricts forwarding to the elected Designated Forwarder
	// only (RFC 7432 Section 14.1.1).
	RedundancyModeSingleActive RedundancyMode = "SingleActive"
)

// DFElectionMode defines the Designated Forwarder election algorithm.
// +kubebuilder:validation:Enum=Default;HighestRandomWeight;Preference
type DFElectionMode string

const (
	// DFElectionModeDefault uses the modulo-based DF election per RFC 7432 Section 8.5.
	DFElectionModeDefault DFElectionMode = "Default"

	// DFElectionModeHighestRandomWeight uses the HRW algorithm per RFC 8584.
	DFElectionModeHighestRandomWeight DFElectionMode = "HighestRandomWeight"

	// DFElectionModePreference uses preference-based DF election per RFC 8584.
	DFElectionModePreference DFElectionMode = "Preference"
)

// DesignatedForwarder configures the DF election parameters for an Ethernet Segment.
//
// +kubebuilder:validation:XValidation:rule="self.electionMode == 'Preference' || !has(self.electionWaitTime)",message="electionWaitTime is only valid when electionMode is Preference"
type DesignatedForwarder struct {
	// ElectionMode selects the DF election algorithm.
	// +kubebuilder:default=Default
	// +optional
	ElectionMode DFElectionMode `json:"electionMode,omitempty"`

	// ElectionWaitTime is the DF election hold timer. The PE waits this
	// duration before selecting the DF based on highest preference.
	// Only applicable when ElectionMode is Preference.
	// +optional
	ElectionWaitTime *metav1.Duration `json:"electionWaitTime,omitempty"`
}

// EthernetSegmentStatus defines the observed state of EthernetSegment.
type EthernetSegmentStatus struct {
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ESI is the realized 10-byte Ethernet Segment Identifier on the device,
	// in colon-separated hex notation. Populated from spec or read back from
	// device when auto-generated.
	// +optional
	ESI string `json:"esi,omitempty"`

	// ESIType is the ESI derivation type parsed from the first byte of ESI.
	// +optional
	ESIType ESIType `json:"esiType,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ethernetsegments
// +kubebuilder:resource:singular=ethernetsegment
// +kubebuilder:resource:shortName=es
// +kubebuilder:printcolumn:name="ESI",type=string,JSONPath=`.status.esi`
// +kubebuilder:printcolumn:name="ESI Type",type=string,JSONPath=`.status.esiType`,priority=1
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Interface",type=string,JSONPath=`.spec.interfaceRef.name`
// +kubebuilder:printcolumn:name="Redundancy Mode",type=string,JSONPath=`.spec.redundancyMode`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EthernetSegment is the Schema for the ethernetsegments API.
type EthernetSegment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec EthernetSegmentSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status EthernetSegmentStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (e *EthernetSegment) GetConditions() []metav1.Condition {
	return e.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (e *EthernetSegment) SetConditions(conditions []metav1.Condition) {
	e.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// EthernetSegmentList contains a list of EthernetSegment.
type EthernetSegmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []EthernetSegment `json:"items"`
}

var (
	EthernetSegmentDependencies   []schema.GroupVersionKind
	ethernetSegmentDependenciesMu sync.Mutex
)

func RegisterEthernetSegmentDependency(gvk schema.GroupVersionKind) {
	ethernetSegmentDependenciesMu.Lock()
	defer ethernetSegmentDependenciesMu.Unlock()
	EthernetSegmentDependencies = append(EthernetSegmentDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &EthernetSegment{}, &EthernetSegmentList{})
		return nil
	})
}
