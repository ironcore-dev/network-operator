// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SyslogSpec defines the desired state of Syslog
type SyslogSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Interface to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Servers is a list of remote log servers to which the device will send logs.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Servers []LogServer `json:"servers"`

	// Facilities is a list of log facilities to configure on the device.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Facilities []LogFacility `json:"facilities"`
}

type LogServer struct {
	// IP address or hostname of the remote log server
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// The servity level of the log messages sent to the server.
	// +required
	Severity Severity `json:"severity"`

	// The name of the vrf used to reach the log server.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	VrfName string `json:"vrfName"`

	// The destination port number for syslog UDP messages to
	// the server. The default is 514.
	// +optional
	// +kubebuilder:default=514
	Port int32 `json:"port"`
}

type LogFacility struct {
	// The name of the log facility.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// The severity level of the log messages for this facility.
	// +required
	Severity Severity `json:"severity"`
}

// Severity represents the severity level of a log message.
// +kubebuilder:validation:Enum=Debug;Info;Notice;Warning;Error;Critical;Alert;Emergency
type Severity string

const (
	SeverityDebug     Severity = "Debug"
	SeverityInfo      Severity = "Info"
	SeverityNotice    Severity = "Notice"
	SeverityWarning   Severity = "Warning"
	SeverityError     Severity = "Error"
	SeverityCritical  Severity = "Critical"
	SeverityAlert     Severity = "Alert"
	SeverityEmergency Severity = "Emergency"
)

// SyslogStatus defines the observed state of Syslog.
type SyslogStatus struct {
	// ServersSummary provides a human-readable summary of the number of log servers.
	// +optional
	ServersSummary string `json:"serversSummary,omitempty"`

	// The conditions are a list of status objects that describe the state of the Banner.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=syslogs
// +kubebuilder:resource:singular=syslog
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Servers",type=string,JSONPath=`.status.serversSummary`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Syslog is the Schema for the syslogs API
type Syslog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec SyslogSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status SyslogStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (sl *Syslog) GetConditions() []metav1.Condition {
	return sl.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (sl *Syslog) SetConditions(conditions []metav1.Condition) {
	sl.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// SyslogList contains a list of Syslog
type SyslogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Syslog `json:"items"`
}

var (
	SyslogDependencies   []schema.GroupVersionKind
	syslogDependenciesMu sync.Mutex
)

func RegisterSyslogDependency(gvk schema.GroupVersionKind) {
	syslogDependenciesMu.Lock()
	defer syslogDependenciesMu.Unlock()
	SyslogDependencies = append(SyslogDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&Syslog{}, &SyslogList{})
}
