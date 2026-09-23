// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=evpninstanceconfigs,verbs=get;list;watch

// EVPNInstanceConfigSpec defines the Cisco NX-OS-specific configuration of an EVPNInstance.
type EVPNInstanceConfigSpec struct {
	// MultisiteIngressReplication controls per-VNI multisite ingress replication.
	// When enabled, BUM traffic for this VNI is replicated to remote VTEP peers
	// in the multisite domain via ingress replication.
	// Typically used on Border Gateway (BGW) nodes.
	// +optional
	// +kubebuilder:default=Disabled
	MultisiteIngressReplication MultisiteIngReplMode `json:"multisiteIngressReplication,omitempty"`
}

// MultisiteIngReplMode defines the per-VNI multisite ingress-replication mode.
// +kubebuilder:validation:Enum=Disabled;Enabled;EnabledOptimized
type MultisiteIngReplMode string

const (
	// MultisiteIngReplDisabled disables multisite ingress replication (default).
	MultisiteIngReplDisabled MultisiteIngReplMode = "Disabled"
	// MultisiteIngReplEnabled enables multisite ingress replication.
	MultisiteIngReplEnabled MultisiteIngReplMode = "Enabled"
	// MultisiteIngReplEnabledOptimized enables optimized multisite ingress replication.
	MultisiteIngReplEnabledOptimized MultisiteIngReplMode = "EnabledOptimized"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=evpninstanceconfigs
// +kubebuilder:resource:singular=evpninstanceconfig
// +kubebuilder:resource:shortName=nxevi

// EVPNInstanceConfig is the Schema for the evpninstanceconfigs API
type EVPNInstanceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of the EVPNInstanceConfig
	// +required
	Spec EVPNInstanceConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// EVPNInstanceConfigList contains a list of EVPNInstanceConfig
type EVPNInstanceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EVPNInstanceConfig `json:"items"`
}

// init registers the EVPNInstanceConfig type with the core v1alpha1 scheme and sets
// itself as a dependency for the EVPNInstance core type.
func init() {
	v1alpha1.RegisterEVPNInstanceDependency(GroupVersion.WithKind("EVPNInstanceConfig"))
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &EVPNInstanceConfig{}, &EVPNInstanceConfigList{})
		return nil
	})
}
