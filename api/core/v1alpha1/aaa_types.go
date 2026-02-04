// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AAASpec defines the desired state of AAA
//
// It models the Authentication, Authorization, and Accounting (AAA) configuration on a network device,
// including TACACS+ server configuration and AAA group/method settings.
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

	// TACACSServers is the list of TACACS+ servers to configure.
	// +optional
	// +listType=map
	// +listMapKey=address
	// +kubebuilder:validation:MaxItems=16
	TACACSServers []TACACSServer `json:"tacacsServers,omitempty"`

	// TACACSGroup is the TACACS+ server group configuration.
	// +optional
	TACACSGroup *TACACSGroup `json:"tacacsGroup,omitempty"`

	// Authentication defines the AAA authentication configuration.
	// +optional
	Authentication *AAAAuthentication `json:"authentication,omitempty"`

	// Authorization defines the AAA authorization configuration.
	// +optional
	Authorization *AAAAuthorization `json:"authorization,omitempty"`

	// Accounting defines the AAA accounting configuration.
	// +optional
	Accounting *AAAAccounting `json:"accounting,omitempty"`
}

// TACACSServer represents a TACACS+ server configuration.
type TACACSServer struct {
	// Address is the IP address or hostname of the TACACS+ server.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// Port is the TCP port of the TACACS+ server.
	// Defaults to 49 if not specified.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=49
	Port int32 `json:"port,omitempty"`

	// KeySecretRef is a reference to a secret containing the shared key for this TACACS+ server.
	// The secret must contain a key specified in the SecretKeySelector.
	// +required
	KeySecretRef SecretKeySelector `json:"keySecretRef"`

	// KeyEncryption specifies the encryption type for the key.
	// Type7 is the Cisco Type 7 encryption (reversible).
	// Type6 is the AES encryption (more secure).
	// Clear means the key is sent in cleartext (not recommended).
	// +optional
	// +kubebuilder:validation:Enum=Type6;Type7;Clear
	// +kubebuilder:default=Type7
	KeyEncryption TACACSKeyEncryption `json:"keyEncryption,omitempty"`

	// Timeout is the timeout in seconds for this TACACS+ server.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=60
	Timeout *int32 `json:"timeout,omitempty"`
}

// TACACSKeyEncryption defines the encryption type for TACACS+ server keys.
// +kubebuilder:validation:Enum=Type6;Type7;Clear
type TACACSKeyEncryption string

const (
	// TACACSKeyEncryptionType6 uses AES encryption (more secure).
	TACACSKeyEncryptionType6 TACACSKeyEncryption = "Type6"
	// TACACSKeyEncryptionType7 uses Cisco Type 7 encryption (reversible).
	TACACSKeyEncryptionType7 TACACSKeyEncryption = "Type7"
	// TACACSKeyEncryptionClear sends the key in cleartext.
	TACACSKeyEncryptionClear TACACSKeyEncryption = "Clear"
)

// TACACSGroup represents a TACACS+ server group configuration.
type TACACSGroup struct {
	// Name is the name of the TACACS+ server group.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Servers is the list of TACACS+ server addresses to include in this group.
	// The addresses must match addresses defined in TACACSServers.
	// +required
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Servers []string `json:"servers"`

	// VRF is the VRF to use for communication with the TACACS+ servers.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	VRF string `json:"vrf,omitempty"`

	// SourceInterface is the source interface to use for communication with the TACACS+ servers.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	SourceInterface string `json:"sourceInterface,omitempty"`
}

// AAAAuthentication defines the AAA authentication configuration.
type AAAAuthentication struct {
	// Login defines authentication methods for login.
	// +optional
	Login *AAAAuthenticationLogin `json:"login,omitempty"`

	// LoginErrorEnable enables login error messages.
	// +optional
	LoginErrorEnable bool `json:"loginErrorEnable,omitempty"`
}

// AAAAuthenticationLogin defines the login authentication methods.
type AAAAuthenticationLogin struct {
	// Default defines the default authentication method list.
	// +optional
	Default *AAAMethodList `json:"default,omitempty"`

	// Console defines the console authentication method list.
	// +optional
	Console *AAAMethodList `json:"console,omitempty"`
}

// AAAAuthorization defines the AAA authorization configuration.
type AAAAuthorization struct {
	// ConfigCommands defines authorization for configuration commands.
	// +optional
	ConfigCommands *AAAAuthorizationConfigCommands `json:"configCommands,omitempty"`
}

// AAAAuthorizationConfigCommands defines authorization for configuration commands.
type AAAAuthorizationConfigCommands struct {
	// Default defines the default authorization method list.
	// +optional
	Default *AAAMethodList `json:"default,omitempty"`
}

// AAAAccounting defines the AAA accounting configuration.
type AAAAccounting struct {
	// Default defines the default accounting method list.
	// +optional
	Default *AAAMethodList `json:"default,omitempty"`
}

// AAAMethodList defines a list of AAA methods to try in order.
type AAAMethodList struct {
	// Methods is the ordered list of authentication/authorization/accounting methods.
	// Methods are tried in order until one succeeds or all fail.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Methods []AAAMethod `json:"methods"`
}

// AAAMethod represents an AAA method.
type AAAMethod struct {
	// Type is the type of AAA method.
	// +required
	// +kubebuilder:validation:Enum=Group;Local;None
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
// +kubebuilder:resource:path=aaas
// +kubebuilder:resource:singular=aaa
// +kubebuilder:resource:shortName=aaa
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="TACACS Group",type=string,JSONPath=`.spec.tacacsGroup.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AAA is the Schema for the aaas API
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
	SchemeBuilder.Register(&AAA{}, &AAAList{})
}
