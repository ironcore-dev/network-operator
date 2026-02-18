// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=aaaconfigs,verbs=get;list;watch

// AAAConfigSpec defines the desired state of AAAConfig
type AAAConfigSpec struct {
	// LoginErrorEnable enables login error messages (NX-OS specific).
	// Maps to: aaa authentication login error-enable
	// +optional
	LoginErrorEnable bool `json:"loginErrorEnable,omitempty"`

	// KeyEncryption specifies the default encryption type for TACACS+ keys.
	// +kubebuilder:validation:Enum=Type6;Type7;Clear
	// +kubebuilder:default=Type7
	KeyEncryption TACACSKeyEncryption `json:"keyEncryption,omitempty"`

	// RADIUSKeyEncryption specifies the default encryption type for RADIUS server keys.
	// +kubebuilder:validation:Enum=Type6;Type7;Clear
	// +kubebuilder:default=Type7
	RADIUSKeyEncryption RADIUSKeyEncryption `json:"radiusKeyEncryption,omitempty"`

	// ConsoleAuthentication defines NX-OS console-specific authentication methods.
	// Maps to: aaa authentication login console <methods>
	// +optional
	ConsoleAuthentication *NXOSMethodList `json:"consoleAuthentication,omitempty"`

	// ConfigCommandsAuthorization defines NX-OS config-commands authorization methods.
	// Maps to: aaa authorization config-commands default <methods>
	// +optional
	ConfigCommandsAuthorization *NXOSMethodList `json:"configCommandsAuthorization,omitempty"`
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

// RADIUSKeyEncryption defines the encryption type for RADIUS server keys.
// +kubebuilder:validation:Enum=Type6;Type7;Clear
type RADIUSKeyEncryption string

const (
	// RADIUSKeyEncryptionType6 uses AES encryption (more secure).
	RADIUSKeyEncryptionType6 RADIUSKeyEncryption = "Type6"
	// RADIUSKeyEncryptionType7 uses Cisco Type 7 encryption (reversible).
	RADIUSKeyEncryptionType7 RADIUSKeyEncryption = "Type7"
	// RADIUSKeyEncryptionClear sends the key in cleartext.
	RADIUSKeyEncryptionClear RADIUSKeyEncryption = "Clear"
)

// NXOSMethodList defines an ordered list of AAA methods for NX-OS specific contexts.
type NXOSMethodList struct {
	// Methods is the ordered list of methods.
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Methods []NXOSMethod `json:"methods"`
}

// NXOSMethod represents a single AAA method in an NX-OS context.
type NXOSMethod struct {
	// Type is the method type.
	// +required
	// +kubebuilder:validation:Enum=Group;Local;None
	Type string `json:"type"`

	// GroupName is the server group name when Type is Group.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	GroupName string `json:"groupName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=aaaconfigs
// +kubebuilder:resource:singular=aaaconfig
// +kubebuilder:resource:shortName=nxaaa

// AAAConfig is the Schema for the aaaconfigs API
type AAAConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec AAAConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// AAAConfigList contains a list of AAAConfig
type AAAConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AAAConfig `json:"items"`
}

func init() {
	v1alpha1.RegisterAAADependency(GroupVersion.WithKind("AAAConfig"))
	SchemeBuilder.Register(&AAAConfig{}, &AAAConfigList{})
}
