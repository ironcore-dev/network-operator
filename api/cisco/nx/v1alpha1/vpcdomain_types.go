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
	// Maps to: "vpcdomain domain <DomainID>"
	// Changing this value will recreate the vPCDomain domain and flap the peer-link and vPCDomains.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	DomainID uint16 `json:"domainId"`

	// AdminState is the administrative state of the vPCDomain domain (enabled/disabled).
	// When disabled, the vPCDomain domain is administratively shut down.
	// Maps to: "vpcdomain domain <id>" being present (enabled) or "no vpcdomain domain <id>" (disabled)
	// +required
	// +kubebuilder:default="enabled"
	// +kubebuilder:validation:Enum=enabled;disabled
	AdminState string `json:"adminState"`

	// RolePriority is the role priority for this vPCDomain domain (1-65535).
	// The switch with the lower role priority becomes the operational primary.
	// Maps to: "role priority <RolePriority>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	RolePriority uint16 `json:"rolePriority"`

	// SystemPriority is the system priority for this vPCDomain domain (1-65535).
	// Used to ensure that the vPCDomain devices are primary devices on LACP. Must match on both peers.
	// Maps to: "system-priority <SystemPriority>"
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	SystemPriority uint16 `json:"systemPriority"`

	// DelayRestoreSVI is the delay in seconds (1-3600) before bringing up interface-vlan (SVI) after peer-link comes up.
	// This prevents traffic blackholing during convergence.
	// Maps to: "delay restore interface-vlan <DelayRestoreSVI>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	DelayRestoreSVI uint16 `json:"delayRestoreSVI"`

	// DelayRestoreVPC is the delay in seconds (1-3600) before bringing up the member ports after the peer-link is restored.
	// Maps to: "delay restore <DelayRestoreVPC>"
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	DelayRestoreVPC uint16 `json:"delayRestoreVPC"`

	// FastConvergence ensures that both SVIs and member ports are shut down simultaneously when the peer-link goes down.
	// This sinchronization helps prevent traffic loss.
	// Maps to: "fast-convergence" when enabled
	// +required
	FastConvergence AdminSt `json:"fastConvergence"`

	// Peer contains the vPCDomain peer configuration including peer-link, keepalive.
	// +required
	Peer Peer `json:"peer"`
}

// AdminSt represents administrative state (enabled/disabled).
// Used for various vPCDomain features.
type AdminSt struct {
	// Enabled indicates whether the feature is administratively enabled (true) or disabled (false).
	// +required
	Enabled bool `json:"enabled"`
}

// Peer defines the vPCDomain peer configuration.
// Encompasses all settings related to the relationship between the two vPCDomain peer switches.
type Peer struct {
	// InterfaceAggregateRef is a reference to an Interface resource with type `Aggregate`.
	// This is a dedicated port-channel between the two switches, that will be configured as the vPCDomain peer-link.
	// and which carries control and data traffic between the two vPCDomain peers.
	// Maps to: "vpcdomain peer-link" configured on the referenced port-channel interface
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
	// Maps to: "peer-switch" when enabled
	// +required
	Switch AdminSt `json:"switch"`

	// Gateway enables peer-gateway functionality on this peer.
	// When enabled, each vPCDomain peer can act as the active gateway for packets destined to the
	// peer's MAC address, improving convergence.
	// Maps to: "peer-gateway" when enabled
	// +required
	Gateway AdminSt `json:"gateway"`

	// Router enables Layer 3 peer-router functionality on this peer.
	// Maps to: "layer3 peer-router" when enabled
	// +required
	Router AdminSt `json:"router"`
}

// KeepAlive defines the vPCDomain keepalive link configuration.
// The keepalive is typically a separate out-of-band link (often over mgmt0) used to monitor
// peer health. It does not carry data traffic.
type KeepAlive struct {
	// Destination is the destination IP address of the vPCDomain peer's keepalive interface.
	// This is the IP address the local switch will send keepalive messages to.
	// Maps to: "peer-keepalive destination <Destination> ..."
	// +kubebuilder:validation:Format=ipv4
	// +required
	Destination string `json:"destination"`

	// Source is the source IP address for keepalive messages.
	// This is the local IP address used to send keepalive packets to the peer.
	// Maps to: "peer-keepalive destination <Destination> source <Source> ..."
	// +kubebuilder:validation:Format=ipv4
	// +required
	Source string `json:"source"`

	// VRFRef is an optional reference to a VRF resource.
	// If specified, the keepalive will use this VRF for routing keepalive packets.
	// Typically used when keepalive is over a management VRF.
	// Maps to: "peer-keepalive destination <Destination> source <Source> vrf <VRFRef.Name>"
	// The VRF must exist on the Device referenced by the parent VPCDomain resource.
	// If omitted, the default VRF is used.
	// +optional
	VRFRef *corev1.LocalObjectReference `json:"vrf,omitempty"`
}

// AutoRecovery holds auto-recovery settings.
// It allows a vPCDomain peer to automatically restore vPCDomain operation after detecting
// that the peer is no longer reachable via keepalive link.
// +kubebuilder:validation:XValidation:rule="self.enabled ? has(self.reloadDelay) : !has(self.reloadDelay)",message="reloadDelay must be set when enabled and absent when disabled"
type AutoRecovery struct {
	// Enabled indicates whether auto-recovery is enabled.
	// When enabled, the switch will wait for ReloadDelay seconds after peer failure
	// before assuming the peer is dead and restoring vPCDomain functionality.
	// Maps to: "auto-recovery" being present (enabled) or absent (disabled)
	Enabled bool `json:"enabled,omitempty"`

	// ReloadDelay is the time in seconds (60-3600) to wait before assuming the peer is dead
	// and automatically recovering vPCDomain operation.
	// Must be set when Enabled is true.
	// Maps to: "auto-recovery reload-delay <ReloadDelay>"
	// +optional
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=3600
	ReloadDelay uint32 `json:"reloadDelay,omitempty"`
}

// VPCDomainStatus defines the observed state of VPCDomain.
type VPCDomainStatus struct {
	// Conditions represent the latest available observations of the VPCDomain's state.
	// Standard conditions include:
	// - Ready: overall readiness of the vPCDomain domain
	// - Configured: whether the vPCDomain configuration was successfully applied to the device
	// - Operational: whether the vPCDomain domain is operationally up (peer-link and keepalive status)
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
	// Possible values:
	// - Primary: This switch is the primary vPCDomain peer (lower role priority or elected)
	// - Secondary: This switch is the secondary vPCDomain peer
	// - Unknown: Role has not been established (e.g., peer-link down, domain not formed)
	// +optional
	Role VPCDomainRole `json:"role,omitempty"`

	// KeepaliveStatus indicates the status of the peer via the keepalive link.
	// +optional
	KeepaliveStatus KeepAliveStatus `json:"keepaliveStatus,omitempty"`

	// PeerStatus indicates the status of the vPCDomain peer-link.
	// +optional
	PeerStatus string `json:"peerStatus,omitempty"`

	// PeerUptime indicates how long the vPCDomain peer has been up and reachable via keepalive.
	// +optional
	PeerUptime metav1.Duration `json:"peerUptime,omitempty"`
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

type KeepAliveStatus string

const (
	KeepAliveStatusUp   KeepAliveStatus = "Up"
	KeepAliveStatusDown KeepAliveStatus = "Down"
)

type PeerStatus string

const (
	PeerStatusUp        PeerStatus = "Up"
	PeerStatusDown      PeerStatus = "Down"
	PeerStatusNotFormed PeerStatus = "NotFormed"
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
// +kubebuilder:printcolumn:name="KeepAliveStatus",type=string,JSONPath=`.status.keepaliveStatus`,priority=1
// +kubebuilder:printcolumn:name="PeerStatus",type=string,JSONPath=`.status.peerStatus`,priority=1
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.status.role`,priority=1
// +kubebuilder:printcolumn:name="PeerUptime",type="date",JSONPath=`.status.peerUptime`,priority=1
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
