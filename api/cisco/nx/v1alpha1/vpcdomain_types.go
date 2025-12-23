// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// VPCDomainSpec defines the desired state of a VPC domain (Virtual Port Channel Domain)
type VPCDomainSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef corev1.LocalObjectReference `json:"deviceRef"`

	// DomainID is the vPCDomain domain ID (1-1000).
	// This uniquely identifies the vPCDomain domain and must match on both peer switches.
	// Changing this value will recreate the vPCDomain domain and flap the peer-link and vPCDomains.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	DomainID int16 `json:"domainId"`

	// AdminState is the administrative state of the vPCDomain domain (enabled/disabled).
	// When disabled, the vPCDomain domain is administratively shut down.
	// +required
	AdminState AdminState `json:"adminState"`

	// RolePriority is the role priority for this vPCDomain domain (1-65535).
	// The switch with the lower role priority becomes the operational primary.
	// Maps to: "role priority <RolePriority>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	RolePriority int32 `json:"rolePriority"`

	// SystemPriority is the system priority for this vPCDomain domain (1-65535).
	// Used to ensure that the vPCDomain devices are primary devices on LACP. Must match on both peers.
	// Maps to: "system-priority <SystemPriority>"
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	SystemPriority int32 `json:"systemPriority"`

	// DelayRestoreSVI is the delay in seconds (1-3600) before bringing up interface-vlan (SVI) after peer-link comes up.
	// This prevents traffic blackholing during convergence.
	// Maps to: "delay restore interface-vlan <DelayRestoreSVI>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	DelayRestoreSVI int16 `json:"delayRestoreSVI"`

	// DelayRestoreVPC is the delay in seconds (1-3600) before bringing up the member ports after the peer-link is restored.
	// Maps to: "delay restore <DelayRestoreVPC>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	DelayRestoreVPC int16 `json:"delayRestoreVPC"`

	// FastConvergence ensures that both SVIs and member ports are shut down simultaneously when the peer-link goes down.
	// This sinchronization helps prevent traffic loss.
	// Maps to: "fast-convergence" when enabled
	// +required
	FastConvergence Enabled `json:"fastConvergence"`

	// Peer contains the vPCDomain peer configuration including peer-link, keepalive.
	// +required
	Peer Peer `json:"peer"`
}

// AdminState represents the administrative state of the peer-link connection (Up/Down).
// +kubebuilder:validation:Enum=Up;Down
type AdminState string

const (
	// AdminStateUp indicates the connection to the peer is administratively enabled.
	AdminStateUp AdminState = "Up"
	// AdminStateDown indicates the connection to the peer is administratively disabled.
	AdminStateDown AdminState = "Down"
)

// Enabled represents a simple enabled/disabled configuration.
type Enabled struct {
	// Enabled indicates whether a configuration property is administratively enabled (true) or disabled (false).
	// +required
	Enabled bool `json:"enabled"`
}

// Peer defines settings to configure peer settings
type Peer struct {
	// VPCId is the peer's VPC ID to use for peer-link communication
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	VPCId int32 `json:"vpcId"`

	// Enabled defines the administrative state of the peer-link (enabled/disabled).
	// +optional
	// +kubebuilder:default="Up"
	AdminState AdminState `json:"adminState,omitempty"`

	// InterfaceAggregateRef is a reference to an Interface resource with type `Aggregate`.
	// This is a dedicated port-channel between the two switches, that will be configured as the vPCDomain peer-link.
	// and which carries control and data traffic between the two vPCDomain peers.
	// +required
	InterfaceAggregateRef corev1.LocalObjectReference `json:"interfaceAggregateRef,omitempty"`

	// KeepAlive defines the out-of-band keepalive configuration.
	// +required
	KeepAlive KeepAlive `json:"keepalive"`

	// AutoRecovery defines auto-recovery settings for restoring vPCDomain after peer failure.
	// +required
	AutoRecovery AutoRecovery `json:"autoRecovery"`

	// Switch enables peer-switch functionality on this peer.
	// When enabled, both vPCDomain peers use the same spanning-tree bridge ID, allowing both
	// to forward traffic for all VLANs without blocking any ports.
	// +required
	Switch Enabled `json:"switch"`

	// Gateway enables peer-gateway functionality on this peer.
	// When enabled, each vPCDomain peer can act as the active gateway for packets destined to the
	// peer's MAC address, improving convergence.
	// +required
	Gateway Enabled `json:"gateway"`

	// L3Router enables Layer 3 peer-router functionality on this peer.
	// +required
	L3Router Enabled `json:"l3router"`
}

// KeepAlive defines the vPCDomain keepalive link configuration.
// The keep-alive is an out-of-band connection (often over mgmt0) used to monitor
// peer health. It does not carry data traffic.
type KeepAlive struct {
	// Destination is the destination IP address of the vPCDomain peer's keepalive interface.
	// This is the IP address the local switch will send keepalive messages to.
	// +kubebuilder:validation:Format=ipv4
	// +required
	Destination string `json:"destination"`

	// Source is the source IP address for keepalive messages.
	// This is the local IP address used to send keepalive packets to the peer.
	// +kubebuilder:validation:Format=ipv4
	// +required
	Source string `json:"source"`

	// VRFRef is an optional reference to a VRF resource.
	// If specified, the keepalive will use this VRF for routing keepalive packets,
	// e.g., the management VRF.
	// If omitted, the default VRF is used.
	// +optional
	VRFRef *corev1.LocalObjectReference `json:"vrfRef,omitempty"`
}

// AutoRecovery holds settings to automatically restore vPCDomain operation after detecting
// that the peer is no longer reachable via the keepalive link.
// +kubebuilder:validation:XValidation:rule="self.enabled ? has(self.reloadDelay) : !has(self.reloadDelay)",message="reloadDelay must be set when enabled and absent when disabled"
type AutoRecovery struct {
	// Enabled indicates whether auto-recovery is enabled.
	// When enabled, the switch will wait for ReloadDelay seconds after peer failure
	// before assuming the peer is dead and restoring vPCDomain functionality.
	Enabled bool `json:"enabled,omitempty"`

	// ReloadDelay is the time in seconds (60-3600) to wait before assuming the peer is dead
	// and automatically attempting to restore the communication with the peer.
	// +optional
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=3600
	ReloadDelay int16 `json:"reloadDelay,omitempty"`
}

// VPCDomainStatus defines the observed state of VPCDomain.
type VPCDomainStatus struct {
	// Conditions represent the latest available observations of the VPCDomain's state.
	// Standard conditions include:
	// - Ready: overall readiness of the vPCDomain domain
	// - Configured: whether the vPCDomain configuration was successfully applied to the device
	// - Operational: whether the vPCDomain domain is operationally up. This condition is true when
	//   the status fields `PeerLinkIfOperStatus`, `KeepAliveStatus`, and `PeerStatus` are all set
	//   to `UP`.
	//
	// For this Cisco model there is not one single unique operational property that reflects the
	// operational status of the vPC domain. The combination of peer status, keepalive status, and
	// the interface used as peer-link determine the overall health and operational condition of
	// the vPC domain.
	//
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DomainID is the vPCDomain domain ID as reported by the device.
	// +optional
	DomainID uint16 `json:"domainId,omitempty"`

	// Role indicates the current operational role of this vPCDomain peer.
	// +optional
	Role VPCDomainRole `json:"role,omitempty"`

	// KeepAliveStatus indicates the status of the peer via the keepalive link.
	// +optional
	KeepAliveStatus Status `json:"keepaliveStatus,omitempty"`

	// KeepAliveStatusMsg provides additional information about the keepalive status as comma separated codes, as returned by the device.
	// +optional
	KeepAliveStatusMsg string `json:"keepaliveStatusMsg,omitempty"`

	// PeerStatus indicates the status of the vPCDomain peer-link in the latest consistency check with the peer. This means that if
	// the adjacency is lost, e.g., due to a shutdown link, the device will not be able to perform such check and the reported status
	// will remain unchanged (with the value of the last check).
	// +optional
	PeerStatus Status `json:"peerStatus,omitempty"`

	// PeerStatusMsg provides additional information about the peer status as comma separated codes, as returned by the device.
	// +optional
	PeerStatusMsg string `json:"peerStatusMsg,omitempty"`

	// PeerUptime indicates how long the vPCDomain peer has been up and reachable via keepalive.
	// +optional
	PeerUptime metav1.Duration `json:"peerUptime,omitempty"`

	// PeerLinkIf is the name of the interface used as the vPCDomain peer-link.
	// +optional
	PeerLinkIf string `json:"peerLinkIf,omitempty"`

	// PeerLinkIfOperStatus is the Operational status of `PeerLinkIf`.
	// +optional
	PeerLinkIfOperStatus Status `json:"peerLinkIfOperStatus,omitempty"`
}

// The VPCDomainRole type represents the operational role of a vPCDomain peer as returned by the device.
type VPCDomainRole string

const (
	VPCDomainRolePrimary                     VPCDomainRole = "Pri"
	VPCDomainRolePrimaryOperationalSecondary VPCDomainRole = "Pri/Sec"
	VPCDomainRoleSecondary                   VPCDomainRole = "Sec"
	VPCDomainRoleSecondaryOperationalPrimary VPCDomainRole = "Sec/Pri"
	VPCDomainRoleUnknown                     VPCDomainRole = "Unknown"
)

type Status string

const (
	StatusUnknown Status = "Unknown"
	StatusUp      Status = "Up"
	StatusDown    Status = "Down"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vpcdomains
// +kubebuilder:resource:singular=vpcdomain
// +kubebuilder:resource:shortName=vpcdomain
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domainId`
// +kubebuilder:printcolumn:name="Enabled",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="PeerStatus",type=string,JSONPath=`.status.peerStatus`,priority=1
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.status.role`,priority=1
// +kubebuilder:printcolumn:name="PeerLinkIf",type="string",JSONPath=".status.peerLinkIf",priority=1
// +kubebuilder:printcolumn:name="PeerLinkIfOperSt",type="string",JSONPath=".status.peerLinkIfOperStatus",priority=1
// +kubebuilder:printcolumn:name="KA-PeerStatus",type=string,JSONPath=`.status.keepaliveStatus`,priority=1
// +kubebuilder:printcolumn:name="KA-PeerUptime",type="string",JSONPath=`.status.peerUptime`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VPCDomain is the Schema for the VPCDomains API
type VPCDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of VPCDomain
	// +required
	Spec VPCDomainSpec `json:"spec,omitempty"`

	// status defines the observed state of VPCDomain
	// +optional
	Status VPCDomainStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (in *VPCDomain) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (in *VPCDomain) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// VPCDomainList contains a list of VPCDomain
type VPCDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCDomain{}, &VPCDomainList{})
}
