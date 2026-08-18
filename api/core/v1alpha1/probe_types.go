// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ProbeSpec defines the desired state of Probe.
// +kubebuilder:validation:XValidation:rule="self.type != 'Ping' || has(self.ping)",message="ping must be specified when type is Ping"
// +kubebuilder:validation:XValidation:rule="self.type == 'Ping' || !has(self.ping)",message="ping must be omitted when type is not Ping"
// +kubebuilder:validation:XValidation:rule="self.type != 'MACTableEntry' || has(self.macTableEntry)",message="macTableEntry must be specified when type is MACTableEntry"
// +kubebuilder:validation:XValidation:rule="self.type == 'MACTableEntry' || !has(self.macTableEntry)",message="macTableEntry must be omitted when type is not MACTableEntry"
// +kubebuilder:validation:XValidation:rule="self.type != 'RoutePresence' || has(self.routePresence)",message="routePresence must be specified when type is RoutePresence"
// +kubebuilder:validation:XValidation:rule="self.type == 'RoutePresence' || !has(self.routePresence)",message="routePresence must be omitted when type is not RoutePresence"
// +kubebuilder:validation:XValidation:rule="self.type != 'VTEPPeerConnectivity' || has(self.vtepPeerConnectivity)",message="vtepPeerConnectivity must be specified when type is VTEPPeerConnectivity"
// +kubebuilder:validation:XValidation:rule="self.type == 'VTEPPeerConnectivity' || !has(self.vtepPeerConnectivity)",message="vtepPeerConnectivity must be omitted when type is not VTEPPeerConnectivity"
type ProbeSpec struct {
	// DeviceRef is a reference to the Device this probe targets.
	// The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this probe.
	// This reference is used to link the Probe to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Schedule is an optional cron expression (e.g., "*/5 * * * *").
	// If omitted, the controller performs a one-shot probe execution only once
	// for the Probe resource; it does not re-execute on subsequent reconciliations.
	// If set, the controller executes the probe periodically according to the schedule.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Type selects which probe assertion to execute.
	// +required
	Type ProbeType `json:"type"`

	// Ping configures an ICMP echo probe.
	// Required when type is Ping, must be omitted otherwise.
	// +optional
	Ping *PingProbe `json:"ping,omitempty"`

	// MACTableEntry configures a MAC address table lookup probe.
	// Required when type is MACTableEntry, must be omitted otherwise.
	// +optional
	MACTableEntry *MACTableEntryProbe `json:"macTableEntry,omitempty"`

	// RoutePresence configures a routing table prefix lookup probe.
	// Required when type is RoutePresence, must be omitted otherwise.
	// +optional
	RoutePresence *RoutePresenceProbe `json:"routePresence,omitempty"`

	// VTEPPeerConnectivity configures a VTEP peer connectivity probe.
	// Required when type is VTEPPeerConnectivity, must be omitted otherwise.
	// +optional
	VTEPPeerConnectivity *VTEPPeerConnectivityProbe `json:"vtepPeerConnectivity,omitempty"`
}

// ProbeType selects which assertion a Probe executes.
// +kubebuilder:validation:Enum=Ping;MACTableEntry;RoutePresence;VTEPPeerConnectivity
type ProbeType string

const (
	// ProbeTypePing sends ICMP echo requests from the device to a target address.
	ProbeTypePing ProbeType = "Ping"
	// ProbeTypeMACTableEntry asserts that a specific MAC address exists in the device's MAC table.
	ProbeTypeMACTableEntry ProbeType = "MACTableEntry"
	// ProbeTypeRoutePresence asserts that an IP prefix exists in a routing table.
	ProbeTypeRoutePresence ProbeType = "RoutePresence"
	// ProbeTypeVTEPPeerConnectivity asserts that expected remote VTEP peers are present and up.
	ProbeTypeVTEPPeerConnectivity ProbeType = "VTEPPeerConnectivity"
)

// PingProbe configures an ICMP echo probe from the device to a target address.
type PingProbe struct {
	// Address is the target IPv4 or IPv6 address to ping.
	// +required
	Address IPAddr `json:"address"`

	// SourceInterface selects the source interface for the ping.
	// The provider uses an address on this interface with the same IP family as Address.
	// If omitted, the device selects the source interface automatically.
	// +optional
	SourceInterface *InterfaceSource `json:"sourceInterface,omitempty"`

	// VRF selects the VRF context in which to execute the ping.
	// If omitted, the ping is executed in the default/global routing table.
	// +optional
	VRF *VRFSource `json:"vrf,omitempty"`

	// Count is the number of ICMP echo requests to send.
	// +optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Count *int32 `json:"count,omitempty"`

	// PacketSize is the ICMP payload size in bytes.
	// Useful for detecting MTU issues in VXLAN overlays.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65507
	PacketSize *int32 `json:"packetSize,omitempty"`

	// Timeout is the maximum time to wait for a reply per echo request.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// MACTableEntryProbe asserts that a specific MAC address exists in the device's forwarding table.
type MACTableEntryProbe struct {
	// MACAddress is the MAC address to look for in the device's MAC table.
	// +required
	// +kubebuilder:validation:Pattern=`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`
	MACAddress string `json:"macAddress"`

	// VLAN constrains the lookup to a specific VLAN.
	// +optional
	VLAN *VLANSource `json:"vlan,omitempty"`
}

// RoutePresenceProbe asserts that an IP prefix exists in the device's routing table.
type RoutePresenceProbe struct {
	// Prefix is the IP prefix to check for (e.g., "10.100.0.0/16", "2001:db8::/32").
	// +required
	Prefix IPPrefix `json:"prefix"`

	// VRF selects the VRF routing table to check.
	// If omitted, the default/global routing table is checked.
	// +optional
	VRF *VRFSource `json:"vrf,omitempty"`
}

// VTEPPeerConnectivityProbe asserts that expected remote VTEP peers are present and operationally up.
type VTEPPeerConnectivityProbe struct {
	// ExpectedPeers lists remote VTEP IP addresses that must be present and up on the device.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=256
	ExpectedPeers []string `json:"expectedPeers"`
}

// ProbeStatus defines the observed state of Probe.
type ProbeStatus struct {
	// LastRunTime is the timestamp of the most recent probe execution,
	// regardless of outcome.
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// NextRunTime is the next time at which the controller intends to
	// execute the probe. Only set when Schedule is configured.
	// +optional
	NextRunTime *metav1.Time `json:"nextRunTime,omitempty"`

	// Ping contains the result of the last Ping probe execution.
	// Only set when the probe type is Ping.
	// +optional
	Ping *PingProbeResult `json:"ping,omitempty"`

	// Conditions represent the current state of the Probe resource.
	// The Ready condition indicates whether the probe assertion passed (True) or failed (False).
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PingProbeResult contains the result of a Ping probe execution.
type PingProbeResult struct {
	// Sent is the number of ICMP echo requests sent.
	// +optional
	Sent int32 `json:"sent,omitempty"`

	// Received is the number of ICMP echo replies received.
	// +optional
	Received int32 `json:"received,omitempty"`

	// MinTime is the minimum round-trip time.
	// +optional
	MinTime *metav1.Duration `json:"minTime,omitempty"`

	// AvgTime is the average round-trip time.
	// +optional
	AvgTime *metav1.Duration `json:"avgTime,omitempty"`

	// MaxTime is the maximum round-trip time.
	// +optional
	MaxTime *metav1.Duration `json:"maxTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=probes
// +kubebuilder:resource:singular=probe
// +kubebuilder:resource:shortName=networkprobe;netprobe
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Run",type=date,JSONPath=`.status.lastRunTime`,priority=1
// +kubebuilder:printcolumn:name="Next Run",type=string,JSONPath=`.status.nextRunTime`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Probe is the Schema for the probes API.
type Probe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ProbeSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ProbeStatus `json:"status,omitzero"`
}

// GetInterfaceReferences returns the names of all Interface resources referenced by this Probe.
func (c *Probe) GetInterfaceReferences() []string {
	if c.Spec.Type == ProbeTypePing && c.Spec.Ping != nil &&
		c.Spec.Ping.SourceInterface != nil && c.Spec.Ping.SourceInterface.InterfaceRef != nil {
		return []string{c.Spec.Ping.SourceInterface.InterfaceRef.Name}
	}
	return nil
}

// GetVLANReferences returns the names of all VLAN resources referenced by this Probe.
func (c *Probe) GetVLANReferences() []string {
	if c.Spec.Type == ProbeTypeMACTableEntry && c.Spec.MACTableEntry != nil &&
		c.Spec.MACTableEntry.VLAN != nil && c.Spec.MACTableEntry.VLAN.VLANRef != nil {
		return []string{c.Spec.MACTableEntry.VLAN.VLANRef.Name}
	}
	return nil
}

// GetVRFReferences returns the names of all VRF resources referenced by this Probe.
func (c *Probe) GetVRFReferences() []string {
	var refs []string
	switch c.Spec.Type {
	case ProbeTypePing:
		if c.Spec.Ping != nil && c.Spec.Ping.VRF != nil && c.Spec.Ping.VRF.VRFRef != nil {
			refs = append(refs, c.Spec.Ping.VRF.VRFRef.Name)
		}
	case ProbeTypeRoutePresence:
		if c.Spec.RoutePresence != nil && c.Spec.RoutePresence.VRF != nil && c.Spec.RoutePresence.VRF.VRFRef != nil {
			refs = append(refs, c.Spec.RoutePresence.VRF.VRFRef.Name)
		}
	case ProbeTypeMACTableEntry, ProbeTypeVTEPPeerConnectivity:
	}
	return refs
}

// GetConditions implements conditions.Getter.
func (c *Probe) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (c *Probe) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ProbeList contains a list of Probe.
type ProbeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Probe `json:"items"`
}

var (
	ProbeListDependencies   []schema.GroupVersionKind
	probeListDependenciesMu sync.Mutex
)

func RegisterProbeListDependency(gvk schema.GroupVersionKind) {
	probeListDependenciesMu.Lock()
	defer probeListDependenciesMu.Unlock()
	ProbeListDependencies = append(ProbeListDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Probe{}, &ProbeList{})
		return nil
	})
}
