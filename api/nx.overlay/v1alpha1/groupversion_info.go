// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the nx.overlay v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=nx.overlay.networking.metal.ironcore.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	// This name is used by applyconfiguration generators (e.g. controller-gen).
	SchemeGroupVersion = schema.GroupVersion{Group: "nx.overlay.networking.metal.ironcore.dev", Version: "v1alpha1"}

	// GroupVersion is an alias for SchemeGroupVersion, for backward compatibility.
	GroupVersion = SchemeGroupVersion

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

const (
	// FinalizerName is the identifier used by EVPNInterconnect controllers to perform cleanup before a resource is deleted.
	FinalizerName = "nx.overlay.networking.metal.ironcore.dev/finalizer"

	// EVPNInterconnectLabel is applied to all sub-resources created by an EVPNInterconnect controller,
	// with the value set to the owning EVPNInterconnect's name.
	EVPNInterconnectLabel = "nx.overlay.networking.metal.ironcore.dev/evpninterconnect"
)

// EVPNInterconnect condition types.
const (
	// UnderlayConvergedCondition reports whether all underlay IGP adjacencies are formed.
	UnderlayConvergedCondition = "UnderlayConverged"

	// OverlayConvergedCondition reports whether all overlay BGP sessions are established.
	OverlayConvergedCondition = "OverlayConverged"
)
