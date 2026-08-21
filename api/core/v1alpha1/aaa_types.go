// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AAASpec defines the desired state of AAA.
//
// It models the Authentication, Authorization, and Accounting (AAA) configuration on a network device.
// +kubebuilder:validation:XValidation:rule="has(self.serverGroups) || has(self.authentication) || has(self.authorization) || has(self.accounting)",message="at least one of serverGroups, authentication, authorization, or accounting must be set"
type AAASpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this AAA.
	// This reference is used to link the AAA to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// ServerGroups is the list of AAA server groups.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=8
	ServerGroups []AAAServerGroup `json:"serverGroups,omitempty"`

	// Authentication defines the AAA authentication method list.
	// +optional
	Authentication *AAAAuthentication `json:"authentication,omitempty"`

	// Authorization defines the AAA authorization method list.
	// +optional
	Authorization *AAAAuthorization `json:"authorization,omitempty"`

	// Accounting defines the AAA accounting method list.
	// +optional
	Accounting *AAAAccounting `json:"accounting,omitempty"`
}

// AAAServerGroupType defines the protocol type of an AAA server group.
// +kubebuilder:validation:Enum=TACACS;RADIUS
type AAAServerGroupType string

const (
	// AAAServerGroupTypeTACACS is a TACACS+ server group.
	AAAServerGroupTypeTACACS AAAServerGroupType = "TACACS"
	// AAAServerGroupTypeRADIUS is a RADIUS server group.
	AAAServerGroupTypeRADIUS AAAServerGroupType = "RADIUS"
)

// AAAServerGroup represents a named group of AAA servers.
// +kubebuilder:validation:XValidation:rule="self.type != 'TACACS' || self.servers.all(s, has(s.tacacs))",message="servers in a TACACS group must have tacacs config"
// +kubebuilder:validation:XValidation:rule="self.type != 'RADIUS' || self.servers.all(s, has(s.radius))",message="servers in a RADIUS group must have radius config"
type AAAServerGroup struct {
	// Name is the name of the server group.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Type is the protocol type of this server group.
	// +required
	Type AAAServerGroupType `json:"type"`

	// Servers is the list of servers in this group.
	// +required
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Servers []AAAServer `json:"servers"`

	// VrfName is the VRF to use for communication with the servers in this group.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	VrfName string `json:"vrfName,omitempty"`

	// SourceInterfaceName is the source interface to use for communication with the servers.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	SourceInterfaceName string `json:"sourceInterfaceName,omitempty"`
}

// AAAServer represents a single AAA server within a group.
type AAAServer struct {
	// Address is the IP address or hostname of the server.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// Timeout is the response timeout for this server.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// TACACS contains TACACS+ specific server configuration.
	// Required when the parent server group type is TACACS.
	// +optional
	TACACS *AAAServerTACACS `json:"tacacs,omitempty"`

	// RADIUS contains RADIUS specific server configuration.
	// Required when the parent server group type is RADIUS.
	// +optional
	RADIUS *AAAServerRADIUS `json:"radius,omitempty"`
}

// AAAServerTACACS contains TACACS+ specific server configuration.
type AAAServerTACACS struct {
	// Port is the TCP port of the TACACS+ server.
	// Defaults to 49 if not specified.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=49
	Port int32 `json:"port"`

	// KeySecretRef is a reference to a secret containing the plain text shared key for this TACACS+ server.
	// The secret must contain a key specified in the SecretKeySelector.
	// +required
	KeySecretRef SecretKeySelector `json:"keySecretRef"`
}

// AAAServerRADIUS contains RADIUS specific server configuration.
type AAAServerRADIUS struct {
	// AuthenticationPort is the UDP port for RADIUS authentication requests.
	// Defaults to 1812 if not specified.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1812
	AuthenticationPort int32 `json:"authenticationPort"`

	// AccountingPort is the UDP port for RADIUS accounting requests.
	// Defaults to 1813 if not specified.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1813
	AccountingPort int32 `json:"accountingPort"`

	// KeySecretRef is a reference to a secret containing the plain text shared key for this RADIUS server.
	// The secret must contain a key specified in the SecretKeySelector.
	// +required
	KeySecretRef SecretKeySelector `json:"keySecretRef"`
}

// AAAAuthentication defines the AAA authentication method list.
type AAAAuthentication struct {
	// Methods is the ordered list of authentication methods.
	// Methods are tried in order until one succeeds or all fail.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Methods []AAAMethod `json:"methods"`
}

// AAAAuthorization defines the AAA authorization method list.
type AAAAuthorization struct {
	// Methods is the ordered list of authorization methods.
	// Methods are tried in order until one succeeds or all fail.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Methods []AAAMethod `json:"methods"`
}

// AAAAccounting defines the AAA accounting method list.
type AAAAccounting struct {
	// Methods is the ordered list of accounting methods.
	// Methods are tried in order until one succeeds or all fail.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Methods []AAAMethod `json:"methods"`
}

// AAAMethod represents an AAA method.
// +kubebuilder:validation:XValidation:rule="self.type != 'Group' || self.groupName != \"\"",message="groupName is required when type is Group"
type AAAMethod struct {
	// Type is the type of AAA method.
	// +required
	Type AAAMethodType `json:"type"`

	// GroupName is the name of the server group when Type is Group.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	GroupName string `json:"groupName,omitempty"`
}

// AAAMethodType defines the type of AAA method.
// +kubebuilder:validation:Enum=Group;Local;None
type AAAMethodType string

const (
	// AAAMethodTypeGroup uses a server group (e.g., TACACS+ group).
	AAAMethodTypeGroup AAAMethodType = "Group"
	// AAAMethodTypeLocal uses the local user database.
	AAAMethodTypeLocal AAAMethodType = "Local"
	// AAAMethodTypeNone allows access without authentication.
	AAAMethodTypeNone AAAMethodType = "None"
)

// AAAStatus defines the observed state of AAA.
type AAAStatus struct {
	// The conditions are a list of status objects that describe the state of the AAA.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=aaa
// +kubebuilder:resource:singular=aaa
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AAA is the Schema for the aaa API
type AAA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec AAASpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status AAAStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (a *AAA) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (a *AAA) SetConditions(conditions []metav1.Condition) {
	a.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// AAAList contains a list of AAA
type AAAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AAA `json:"items"`
}

var (
	AAADependencies   []schema.GroupVersionKind
	aaaDependenciesMu sync.Mutex
)

func RegisterAAADependency(gvk schema.GroupVersionKind) {
	aaaDependenciesMu.Lock()
	defer aaaDependenciesMu.Unlock()
	AAADependencies = append(AAADependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &AAA{}, &AAAList{})
		return nil
	})
}
