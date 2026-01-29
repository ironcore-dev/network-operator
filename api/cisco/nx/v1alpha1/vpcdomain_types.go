// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// VPCDomainSpec defines the desired state of a vPC domain (Virtual Port Channel Domain)
type VPCDomainSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef v1alpha1.LocalObjectReference `json:"deviceRef"`

	// DomainID is the vPC domain ID (1-1000).
	// This uniquely identifies the vPC domain and must match on both peer switches.
	// Changing this value will recreate the vPC domain and flap the peer-link.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	DomainID int16 `json:"domainId"`

	// AdminState is the administrative state of the vPC domain (enabled/disabled).
	// When disabled, the vPC domain is administratively shut down.
	// +optional
	// +kubebuilder:default=Up
	AdminState v1alpha1.AdminState `json:"adminState"`

	// RolePriority is the role priority for this vPC domain (1-65535).
	// The switch with the lower role priority becomes the operational primary.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=32667
	RolePriority int32 `json:"rolePriority"`

	// SystemPriority is the system priority for this vPC domain (1-65535).
	// Used to ensure that the vPC domain devices are primary devices on LACP. Must match on both peers.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=32667
	SystemPriority int32 `json:"systemPriority"`

	// DelayRestoreSVI is the delay in seconds (1-3600) before bringing up interface-vlan (SVI) after peer-link comes up.
	// This prevents traffic blackholing during convergence.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=10
	DelayRestoreSVI int16 `json:"delayRestoreSVI"`

	// DelayRestoreVPC is the delay in seconds (1-3600) before bringing up the member ports after the peer-link is restored.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=30
	DelayRestoreVPC int16 `json:"delayRestoreVPC"`

	// FastConvergence ensures that both SVIs and member ports are shut down simultaneously when the peer-link goes down.
	// This synchronization helps prevent traffic loss.
	// +optional
	// +kubebuilder:default={enabled:false}
	FastConvergence Enabled `json:"fastConvergence"`

	// Peer contains the vPC's domain peer configuration including peer-link, keepalive.
	// +required
	Peer Peer `json:"peer"`
}

// Enabled represents a simple enabled/disabled configuration.
type Enabled struct {
	// Enabled indicates whether a configuration property is administratively enabled (true) or disabled (false).
	// +required
	Enabled bool `json:"enabled"`
}

// Peer defines settings to configure peer settings
type Peer struct {
	// AdminState defines the administrative state of the peer-link.
	// +optional
	// +kubebuilder:default=Up
	AdminState v1alpha1.AdminState `json:"adminState"`

	// InterfaceRef is a reference to an Interface resource and identifies the interface to be used as the vPC domain's peer-link.
	// This interface carries control and data traffic between the two vPC domain peers.
	// It is usually dedicated port-channel, but it can also be a single physical interface.
	// +required
	InterfaceRef v1alpha1.LocalObjectReference `json:"interfaceRef"`

	// KeepAlive defines the out-of-band keepalive configuration.
	// +required
	KeepAlive KeepAlive `json:"keepalive"`

	// AutoRecovery defines auto-recovery settings for restoring vPC domain after peer failure.
	// +optional
	AutoRecovery *AutoRecovery `json:"autoRecovery"`

	// Switch enables peer-switch functionality on this peer.
	// When enabled, both vPC domain peers use the same spanning-tree bridge ID, allowing both
	// to forward traffic for all VLANs without blocking any ports.
	// +optional
	// +kubebuilder:default={enabled:false}
	Switch Enabled `json:"switch"`

	// Gateway enables peer-gateway functionality on this peer.
	// When enabled, each vPC domain peer can act as the active gateway for packets destined to the
	// peer's MAC address, improving convergence.
	// +optional
	// +kubebuilder:default={enabled:false}
	Gateway Enabled `json:"gateway"`

	// L3Router enables Layer 3 peer-router functionality on this peer.
	// +optional
	// +kubebuilder:default={enabled:false}
	L3Router Enabled `json:"l3router"`
}

// KeepAlive defines the vPCDomain keepalive link configuration.
// The keep-alive is an out-of-band connection (often over mgmt0) used to monitor
// peer health. It does not carry data traffic.
type KeepAlive struct {
	// Destination is the destination IP address of the vPC's domain peer keepalive interface.
	// This is the IP address the local switch will send keepalive messages to.
	// +kubebuilder:validation:Format=ipv4
	// +required
	Destination string `json:"destination"`

	// Source is the source IP address for keepalive messages.
	// This is the local IP address used to send keepalive packets to the peer.
	// +kubebuilder:validation:Format=ipv4
	// +required
	Source string `json:"source"`

	// VRFRef is an optional reference to a VRF resource, e.g., the management VRF.
	// If specified, the switch sends keepalive packets throughout this VRF.
	// If omitted, the management VRF is used.
	// +optional
	VRFRef *v1alpha1.LocalObjectReference `json:"vrfRef,omitempty"`
}

// AutoRecovery holds settings to automatically restore vPC domain's operation after detecting
// that the peer is no longer reachable via the keepalive link.
// +kubebuilder:validation:XValidation:rule="self.enabled ? has(self.reloadDelay) : !has(self.reloadDelay)",message="reloadDelay must be set when enabled and absent when disabled"
type AutoRecovery struct {
	// Enabled indicates whether auto-recovery is enabled.
	// When enabled, the switch will wait for ReloadDelay seconds after peer failure
	// before assuming the peer is dead and restoring the vPC's domain functionality.
	// +required
	Enabled bool `json:"enabled"`

	// ReloadDelay is the time in seconds (60-3600) to wait before assuming the peer is dead
	// and automatically attempting to restore the communication with the peer.
	// +optional
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=240
	ReloadDelay int16 `json:"reloadDelay"`
}

// VPCDomainStatus defines the observed state of VPCDomain.
type VPCDomainStatus struct {
	// Conditions represent the latest available observations about the vPCDomain state.
	// Standard conditions include:
	// - Ready: overall readiness of the vPC domain
	// - Configured: whether the vPCDomain configuration was successfully applied to the device
	// - Operational: whether the vPC domain is operationally up. This condition is true when
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

	// Role indicates the current operational role of this vPC domain peer.
	// +optional
	Role VPCDomainRole `json:"role,omitempty"`

	// KeepAliveStatus indicates the status of the peer via the keepalive link.
	// +optional
	KeepAliveStatus Status `json:"keepaliveStatus,omitempty"`

	// KeepAliveStatusMsg provides additional information about the keepalive status, a list of strings reported by the device.
	// +optional
	KeepAliveStatusMsg []string `json:"keepaliveStatusMsg,omitempty"`

	// PeerStatus indicates the status of the vPC domain peer-link in the latest consistency check with the peer. This means that if
	// the adjacency is lost, e.g., due to a shutdown link, the device will not be able to perform such check and the reported status
	// will remain unchanged (with the value of the last check).
	// +optional
	PeerStatus Status `json:"peerStatus,omitempty"`

	// PeerStatusMsg provides additional information about the peer status, a list of strings reported by the device.
	// +optional
	PeerStatusMsg []string `json:"peerStatusMsg,omitempty"`

	// PeerUptime indicates how long the vPC domain peer has been up and reachable via keepalive.
	// +optional
	PeerUptime metav1.Duration `json:"peerUptime,omitempty,omitzero"`

	// PeerLinkIf is the name of the interface used as the vPC domain peer-link.
	// +optional
	PeerLinkIf string `json:"peerLinkIf,omitempty"`

	// PeerLinkIfOperStatus is the Operational status of `PeerLinkIf`.
	// +optional
	PeerLinkIfOperStatus Status `json:"peerLinkIfOperStatus,omitempty"`
}

// The VPCDomainRole type represents the operational role of a vPC domain peer as returned by the device.
type VPCDomainRole string

const (
	VPCDomainRolePrimary                     VPCDomainRole = "Primary"
	VPCDomainRolePrimaryOperationalSecondary VPCDomainRole = "Primary/Secondary"
	VPCDomainRoleSecondary                   VPCDomainRole = "Secondary"
	VPCDomainRoleSecondaryOperationalPrimary VPCDomainRole = "Secondary/Primary"
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
// +kubebuilder:printcolumn:name="Admin State",type=string,JSONPath=`.spec.adminState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`,priority=1
// +kubebuilder:printcolumn:name="Operational",type=string,JSONPath=`.status.conditions[?(@.type=="Operational")].status`,priority=1
// +kubebuilder:printcolumn:name="PeerStatus",type=string,JSONPath=`.status.peerStatus`,priority=1
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.status.role`,priority=1
// +kubebuilder:printcolumn:name="PeerLinkIf",type="string",JSONPath=".status.peerLinkIf",priority=1
// +kubebuilder:printcolumn:name="PeerLinkIfOperSt",type="string",JSONPath=".status.peerLinkIfOperStatus",priority=1
// +kubebuilder:printcolumn:name="KeepalivePeerStatus",type=string,JSONPath=`.status.keepaliveStatus`,priority=1
// +kubebuilder:printcolumn:name="KeepalivePeerUptime",type="string",JSONPath=`.status.peerUptime`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VPCDomain is the Schema for the VPCDomains API
type VPCDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of VPCDomain resource
	// +required
	Spec VPCDomainSpec `json:"spec,omitempty"`

	// status defines the observed state of VPCDomain resource
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

// VPCDomainList contains a list of VPCDomain resources
type VPCDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCDomain{}, &VPCDomainList{})
}
