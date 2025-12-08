// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InterfaceSpec defines the desired state of Interface.
// +kubebuilder:validation:XValidation:rule="!has(self.switchport) || !has(self.ipv4)", message="switchport and ipv4 are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="self.type != 'Loopback' || !has(self.switchport)", message="switchport must not be specified for interfaces of type Loopback"
// +kubebuilder:validation:XValidation:rule="self.type == 'Physical' || !has(self.ipv4) || !has(self.ipv4.unnumbered)", message="unnumbered ipv4 configuration can only be used for interfaces of type Physical"
// +kubebuilder:validation:XValidation:rule="self.type != 'Aggregate' || has(self.aggregation)", message="aggregation must be specified for interfaces of type Aggregate"
// +kubebuilder:validation:XValidation:rule="self.type == 'Aggregate' || !has(self.aggregation)", message="aggregation must only be specified on interfaces of type Aggregate"
// +kubebuilder:validation:XValidation:rule="self.type != 'Aggregate' || !has(self.ipv4)", message="ipv4 must not be specified for interfaces of type Aggregate"
// +kubebuilder:validation:XValidation:rule="self.type != 'RoutedVLAN' || has(self.vlanRef)", message="vlanRef must be specified for interfaces of type RoutedVLAN"
// +kubebuilder:validation:XValidation:rule="self.type == 'RoutedVLAN' || !has(self.vlanRef)", message="vlanRef must only be specified on interfaces of type RoutedVLAN"
// +kubebuilder:validation:XValidation:rule="self.type != 'RoutedVLAN' || !has(self.switchport)", message="switchport must not be specified for interfaces of type RoutedVLAN"
// +kubebuilder:validation:XValidation:rule="self.type != 'RoutedVLAN' || !has(self.aggregation)", message="aggregation must not be specified for interfaces of type RoutedVLAN"
// +kubebuilder:validation:XValidation:rule="self.type == 'RoutedVLAN' || !has(self.ipv4) || !self.ipv4.anycastGateway", message="anycastGateway can only be enabled for interfaces of type RoutedVLAN"
// +kubebuilder:validation:XValidation:rule="self.type != 'Aggregate' || !has(self.vrfRef)", message="vrfRef must not be specified for interfaces of type Aggregate"
// +kubebuilder:validation:XValidation:rule="self.type != 'Physical' || !has(self.switchport) || !has(self.vrfRef)", message="vrfRef must not be specified for Physical interfaces with switchport configuration"
// +kubebuilder:validation:XValidation:rule="self.type != 'Aggregate' || !has(self.bfd)", message="bfd must not be specified for interfaces of type Aggregate"
// +kubebuilder:validation:XValidation:rule="!has(self.bfd) || !has(self.switchport)", message="bfd must not be specified for interfaces with switchport configuration"
type InterfaceSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the name of the interface.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// AdminState indicates whether the interface is administratively up or down.
	// +required
	AdminState AdminState `json:"adminState"`

	// Description provides a human-readable description of the interface.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`

	// Type indicates the type of the interface.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	Type InterfaceType `json:"type"`

	// MTU (Maximum Transmission Unit) specifies the size of the largest packet that can be sent over the interface.
	// +optional
	// +kubebuilder:validation:Minimum=576
	// +kubebuilder:validation:Maximum=9216
	MTU int32 `json:"mtu,omitempty"`

	// Switchport defines the switchport configuration for the interface.
	// This is only applicable for Ethernet and Aggregate interfaces.
	// +optional
	Switchport *Switchport `json:"switchport,omitempty"`

	// IPv4 defines the IPv4 configuration for the interface.
	// +optional
	IPv4 *InterfaceIPv4 `json:"ipv4,omitempty"`

	// Aggregation defines the aggregation (bundle) configuration for the interface.
	// This is only applicable for interfaces of type Aggregate.
	// +optional
	Aggregation *Aggregation `json:"aggregation,omitempty"`

	// VlanRef is a reference to the VLAN resource that this interface provides routing for.
	// This is only applicable for interfaces of type RoutedVLAN.
	// The referenced VLAN must exist in the same namespace.
	// +optional
	VlanRef *LocalObjectReference `json:"vlanRef,omitempty"`

	// VrfRef is a reference to the VRF resource that this interface belongs to.
	// If not specified, the interface will be part of the default VRF.
	// This is only applicable for Layer 3 interfaces.
	// The referenced VRF must exist in the same namespace.
	// +optional
	VrfRef *LocalObjectReference `json:"vrfRef,omitempty"`

	// BFD defines the Bidirectional Forwarding Detection configuration for the interface.
	// BFD is only applicable for Layer 3 interfaces (Physical, Loopback, RoutedVLAN).
	// +optional
	BFD *BFD `json:"bfd,omitempty"`
}

// AdminState represents the administrative state of the interface.
// +kubebuilder:validation:Enum=Up;Down
type AdminState string

const (
	// AdminStateUp indicates that the interface is administratively set up.
	AdminStateUp AdminState = "Up"
	// AdminStateDown indicates that the interface is administratively set down.
	AdminStateDown AdminState = "Down"
)

// InterfaceType represents the type of the interface.
// +kubebuilder:validation:Enum=Physical;Loopback;Aggregate;RoutedVLAN
type InterfaceType string

const (
	// InterfaceTypePhysical indicates that the interface is a physical/ethernet interface.
	InterfaceTypePhysical InterfaceType = "Physical"
	// InterfaceTypeLoopback indicates that the interface is a loopback interface.
	InterfaceTypeLoopback InterfaceType = "Loopback"
	// InterfaceTypeAggregate indicates that the interface is an aggregate (bundle) interface.
	InterfaceTypeAggregate InterfaceType = "Aggregate"
	// InterfaceTypeRoutedVLAN indicates that the interface is a routed VLAN interface (SVI/IRB).
	InterfaceTypeRoutedVLAN InterfaceType = "RoutedVLAN"
)

// Switchport defines the switchport configuration for an interface.
// +kubebuilder:validation:XValidation:rule="self.mode != 'Access' || has(self.accessVlan)", message="accessVlan must be specified when mode is Access"
type Switchport struct {
	// Mode defines the switchport mode, such as access or trunk.
	// +required
	Mode SwitchportMode `json:"mode"`

	// AccessVlan specifies the VLAN ID for access mode switchports.
	// Only applicable when Mode is set to "Access".
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	AccessVlan int32 `json:"accessVlan,omitempty"`

	// NativeVlan specifies the native VLAN ID for trunk mode switchports.
	// Only applicable when Mode is set to "Trunk".
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	NativeVlan int32 `json:"nativeVlan,omitempty"`

	// AllowedVlans is a list of VLAN IDs that are allowed on the trunk port.
	// If not specified, all VLANs (1-4094) are allowed.
	// Only applicable when Mode is set to "Trunk".
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:Minimum=1
	// +kubebuilder:validation:items:Maximum=4094
	AllowedVlans []int32 `json:"allowedVlans,omitempty"`
}

// SwitchportMode represents the switchport mode of an interface.
// +kubebuilder:validation:Enum=Access;Trunk
type SwitchportMode string

const (
	// SwitchportModeAccess indicates that the switchport is in access mode.
	SwitchportModeAccess SwitchportMode = "Access"
	// SwitchportModeTrunk indicates that the switchport is in trunk mode.
	SwitchportModeTrunk SwitchportMode = "Trunk"
)

// InterfaceIPv4 defines the IPv4 configuration for an interface.
// +kubebuilder:validation:XValidation:rule="!has(self.addresses) || !has(self.unnumbered)", message="addresses and unnumbered are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.unnumbered) || !self.anycastGateway", message="anycastGateway and unnumbered are mutually exclusive"
type InterfaceIPv4 struct {
	// Addresses defines the list of IPv4 addresses assigned to the interface.
	// The first address in the list is considered the primary address,
	// and any additional addresses are considered secondary addresses.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Addresses []IPPrefix `json:"addresses,omitempty"`

	// Unnumbered defines the unnumbered interface configuration.
	// When specified, the interface borrows the IP address from another interface.
	// +optional
	Unnumbered *InterfaceIPv4Unnumbered `json:"unnumbered,omitempty"`

	// AnycastGateway enables distributed anycast gateway functionality.
	// When enabled, this interface uses the virtual MAC configured in the
	// device's NVE resource for active-active default gateway redundancy.
	// Only applicable for RoutedVLAN interfaces in EVPN/VXLAN fabrics.
	// +optional
	// +kubebuilder:default=false
	AnycastGateway bool `json:"anycastGateway,omitempty"`
}

// InterfaceIPv4Unnumbered defines the unnumbered interface configuration.
// An unnumbered interface borrows the IP address from another interface,
// allowing the interface to function without its own IP address assignment.
type InterfaceIPv4Unnumbered struct {
	// InterfaceRef is a reference to the interface from which to borrow the IP address.
	// The referenced interface must exist and have at least one IPv4 address configured.
	// +required
	InterfaceRef LocalObjectReference `json:"interfaceRef"`
}

// BFD defines the Bidirectional Forwarding Detection configuration for an interface.
type BFD struct {
	// Enabled indicates whether BFD is enabled on the interface.
	// +required
	Enabled bool `json:"enabled"`

	// DesiredMinimumTxInterval is the minimum interval between transmission of BFD control
	// packets that the operator desires. This value is advertised to the peer.
	// The actual interval used is the maximum of this value and the remote
	// required-minimum-receive interval value.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	DesiredMinimumTxInterval *metav1.Duration `json:"desiredMinimumTxInterval,omitempty"`

	// RequiredMinimumReceive is the minimum interval between received BFD control packets
	// that this system should support. This value is advertised to the remote peer to
	// indicate the maximum frequency between BFD control packets that is acceptable
	// to the local system.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	RequiredMinimumReceive *metav1.Duration `json:"requiredMinimumReceive,omitempty"`

	// DetectionMultiplier is the number of packets that must be missed to declare
	// this session as down. The detection interval for the BFD session is calculated
	// by multiplying the value of the negotiated transmission interval by this value.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	DetectionMultiplier *int32 `json:"detectionMultiplier,omitempty"`
}

type Aggregation struct {
	// MemberInterfaceRefs is a list of interface references that are part of the aggregate interface.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	MemberInterfaceRefs []LocalObjectReference `json:"memberInterfaceRefs"`

	// ControlProtocol defines the lacp configuration for the aggregate interface.
	// +optional
	// +kubebuilder:default={mode: Active}
	ControlProtocol ControlProtocol `json:"controlProtocol"`

	// Multichassis defines the multichassis configuration for the aggregate interface.
	// +optional
	MultiChassis *MultiChassis `json:"multichassis,omitempty"`
}

type ControlProtocol struct {
	// Mode defines the LACP mode for the aggregate interface.
	// +required
	Mode LACPMode `json:"mode"`
}

// LACPMode represents the LACP mode of an interface.
// +kubebuilder:validation:Enum=Active;Passive
type LACPMode string

const (
	// LACPModeActive indicates that LACP is in active mode.
	LACPModeActive LACPMode = "Active"
	// LACPModePassive indicates that LACP is in passive mode.
	LACPModePassive LACPMode = "Passive"
)

type MultiChassis struct {
	// Enabled indicates whether the aggregate interface is part of a multichassis setup.
	// +required
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// ID is the multichassis identifier.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	ID int16 `json:"id"`
}

// InterfaceStatus defines the observed state of Interface.
type InterfaceStatus struct {
	// The conditions are a list of status objects that describe the state of the Interface.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MemberOf references the aggregate interface this interface is a member of, if any.
	// This field only applies to physical interfaces that are part of an aggregate interface.
	// +optional
	MemberOf *LocalObjectReference `json:"memberOf,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=interfaces
// +kubebuilder:resource:singular=interface
// +kubebuilder:resource:shortName=int
// +kubebuilder:printcolumn:name="Interface",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
// +kubebuilder:printcolumn:name="MTU",type=string,JSONPath=`.spec.mtu`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Interface is the Schema for the interfaces API.
type Interface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec InterfaceSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status InterfaceStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (in *Interface) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (in *Interface) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// InterfaceList contains a list of Interface.
type InterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Interface `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Interface{}, &InterfaceList{})
}
