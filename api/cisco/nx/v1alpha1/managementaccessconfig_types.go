// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=managementaccessconfigs,verbs=get;list;watch

// ManagementAccessConfigSpec defines the desired state of ManagementAccessConfig
type ManagementAccessConfigSpec struct {
	// Console defines the configuration for the terminal console access on the device.
	// +optional
	// +kubebuilder:default={timeout:"10m"}
	Console Console `json:"console,omitzero"`

	// SSH defines the SSH server configuration for the VTY terminal access on the device.
	// +optional
	SSH SSH `json:"ssh,omitzero"`
}

type Console struct {
	// Timeout defines the inactivity timeout for console sessions.
	// If a session is inactive for the specified duration, it will be automatically disconnected.
	// The format is a string representing a duration (e.g., "10m" for 10 minutes).
	// +optional
	// +kubebuilder:default="10m"
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	Timeout metav1.Duration `json:"timeout,omitzero"`
}

type SSH struct {
	// AccessControlListName defines the name of the access control list (ACL) to apply for incoming
	// SSH connections on the VTY terminal. The ACL must be configured separately on the device.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	AccessControlListName string `json:"accessControlListName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=managementaccessconfigs
// +kubebuilder:resource:singular=managementaccessconfig
// +kubebuilder:resource:shortName=nxmgmt;nxmgmtaccess

// ManagementAccessConfig is the Schema for the managementaccessconfigs API
type ManagementAccessConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ManagementAccessConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ManagementAccessConfigList contains a list of ManagementAccessConfig
type ManagementAccessConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagementAccessConfig `json:"items"`
}

func init() {
	v1alpha1.RegisterManagementAccessDependency(GroupVersion.WithKind("ManagementAccessConfig"))
	SchemeBuilder.Register(&ManagementAccessConfig{}, &ManagementAccessConfigList{})
}
