// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ConsoleConnectionSpec defines the desired state of ConsoleConnection.
type ConsoleConnectionSpec struct {
	// DeviceRef is a reference to the Device this console connection targets.
	// The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// Endpoint contains the console server connection details.
	// +required
	Endpoint ConsoleEndpoint `json:"endpoint"`

	// Verification configures how the controller confirms the serial
	// line is alive and connected to the expected device.
	// +optional
	Verification ConsoleVerification `json:"verification,omitempty"`

	// Schedule is an optional cron expression (e.g., "*/5 * * * *").
	// If omitted, the controller performs a one-shot check only once
	// for the resource; it does not re-execute on subsequent reconciliations.
	// If set, the controller checks periodically according to the schedule.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Timeout is the maximum duration the controller waits for output on
	// the serial line before declaring the connection dead.
	// +kubebuilder:default="30s"
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// ConsoleEndpoint contains the console server connection details.
type ConsoleEndpoint struct {
	// Address is the console server address in IP:Port format.
	// The port identifies the serial line on the console server.
	// +required
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}:\d{1,5}$`
	Address string `json:"address"`

	// Protocol is the connection protocol.
	// +kubebuilder:default=SSH
	// +optional
	Protocol ConsoleProtocol `json:"protocol,omitempty"`

	// SecretRef references a kubernetes.io/basic-auth secret containing
	// 'username' and 'password' for the console server.
	// +required
	SecretRef SecretReference `json:"secretRef"`
}

// ConsoleProtocol is the connection protocol used to reach the console server.
// +kubebuilder:validation:Enum=SSH
type ConsoleProtocol string

const ConsoleProtocolSSH ConsoleProtocol = "SSH"

// ConsoleVerification configures how the controller confirms the serial
// line is alive and connected to the expected device.
// +kubebuilder:validation:XValidation:rule="self.strategy != 'SendChar' || has(self.char)",message="char must be specified when strategy is SendChar"
// +kubebuilder:validation:XValidation:rule="self.strategy == 'SendChar' || !has(self.char)",message="char must be omitted when strategy is not SendChar"
type ConsoleVerification struct {
	// Strategy selects how the controller stimulates the serial line.
	//
	//   Wait     — passively wait for output without sending anything.
	//   SendCRLF — send a carriage-return/line-feed to trigger a prompt or response.
	//   SendChar — send a single printable character to trigger a response.
	//
	// Defaults to SendCRLF.
	// +kubebuilder:default=SendCRLF
	// +optional
	Strategy ConsoleVerificationStrategy `json:"strategy,omitempty"`

	// Char is the character to send when Strategy is SendChar.
	// Ignored for other strategies.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1
	Char *string `json:"char,omitempty"`

	// Expect configures what the controller looks for in the serial output.
	// If omitted, the controller matches the device hostname or serial number
	// from Device.Status.
	// +optional
	Expect *ConsoleExpect `json:"expect,omitempty"`
}

// ConsoleVerificationStrategy selects how the controller stimulates the serial line.
// +kubebuilder:validation:Enum=Wait;SendCRLF;SendChar
type ConsoleVerificationStrategy string

const (
	ConsoleVerificationWait     ConsoleVerificationStrategy = "Wait"
	ConsoleVerificationSendCRLF ConsoleVerificationStrategy = "SendCRLF"
	ConsoleVerificationSendChar ConsoleVerificationStrategy = "SendChar"
)

// ConsoleExpect configures what the controller looks for in the serial output.
type ConsoleExpect struct {
	// String is a literal string to match in the serial output.
	// +optional
	String *string `json:"string,omitempty"`

	// Regex is a regular expression to match in the serial output.
	// +optional
	Regex *string `json:"regex,omitempty"`
}

// ConsoleConnectionStatus defines the observed state of ConsoleConnection.
type ConsoleConnectionStatus struct {
	// LastCheckTime is the timestamp of the most recent check.
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// NextCheckTime is the next scheduled check. Only set when Schedule is configured.
	// +optional
	NextCheckTime *metav1.Time `json:"nextCheckTime,omitempty"`

	// Conditions represent the current state of the ConsoleConnection resource.
	// The Ready condition reports the health of the console connection.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Console connection Ready condition reasons.
const (
	// ConsoleServerUnreachableReason indicates the console server could not be reached.
	ConsoleServerUnreachableReason = "ConsoleServerUnreachable"
	// ConsoleServerAuthFailureReason indicates authentication to the console server failed.
	ConsoleServerAuthFailureReason = "ConsoleServerAuthFailure"
	// ConsoleDeadReason indicates the console server was reachable but no output was received.
	ConsoleDeadReason = "Dead"
	// ConsoleAliveReason indicates output was received but the expected string was not matched.
	ConsoleAliveReason = "Alive"
	// ConsoleVerifiedReason indicates the expected output was matched, confirming device identity.
	ConsoleVerifiedReason = "Verified"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=consoleconnections
// +kubebuilder:resource:singular=consoleconnection
// +kubebuilder:resource:shortName=conn;console;connection
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`,priority=1
// +kubebuilder:printcolumn:name="Last Check",type=date,JSONPath=`.status.lastCheckTime`,priority=1
// +kubebuilder:printcolumn:name="Next Check",type=string,JSONPath=`.status.nextCheckTime`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ConsoleConnection is the Schema for the consoleconnections API.
type ConsoleConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ConsoleConnectionSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ConsoleConnectionStatus `json:"status,omitzero"`
}

// GetConditions implements conditions.Getter.
func (c *ConsoleConnection) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (c *ConsoleConnection) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ConsoleConnectionList contains a list of ConsoleConnection.
type ConsoleConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ConsoleConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ConsoleConnection{}, &ConsoleConnectionList{})
		return nil
	})
}
