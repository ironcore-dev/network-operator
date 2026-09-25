// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=pimconfigs,verbs=get;list;watch

// PIMConfigSpec defines the Cisco NX-OS specific PIM configuration.
type PIMConfigSpec struct {
	// LogNeighborChanges enables logging when a PIM neighbor is added or removed.
	// +optional
	LogNeighborChanges *bool `json:"logNeighborChanges,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=pimconfigs
// +kubebuilder:resource:singular=pimconfig
// +kubebuilder:resource:shortName=nxpim

// PIMConfig is the Schema for the PIMConfig API
type PIMConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PIMConfig
	// +required
	Spec PIMConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// PIMConfigList contains a list of PIMConfigs
type PIMConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PIMConfig `json:"items"`
}

// init registers the PIMConfig type with the scheme and sets
// itself as a dependency for the PIM core type.
func init() {
	v1alpha1.RegisterPIMDependency(GroupVersion.WithKind("PIMConfig"))
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &PIMConfig{}, &PIMConfigList{})
		return nil
	})
}
