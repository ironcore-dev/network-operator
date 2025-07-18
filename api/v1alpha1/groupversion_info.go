// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the networking.cloud.sap v1alpha1 API group.
// +kubebuilder:validation:Required
// +kubebuilder:object:generate=true
// +groupName=networking.cloud.sap
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "networking.cloud.sap", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// WatchLabel is a label that can be applied to any Network API object.
//
// Controllers which allow for selective reconciliation may check this label and proceed
// with reconciliation of the object only if this label and a configured value is present.
const WatchLabel = "networking.cloud.sap/watch-filter"

// FinalizerName is the identifier used by the controllers to perform cleanup before a resource is deleted.
// It is added when the resource is created and ensures that the controller can handle teardown logic
// (e.g., deleting external dependencies) before Kubernetes finalizes the deletion.
const FinalizerName = "networking.cloud.sap/finalizer"

// Condition types that are used across different objects.
const (
	// Ready is the top-level status condition that reports if an object is ready.
	// This condition indicates whether the resource is ready to be used and will be calculated by the
	// controller based on child conditions, if present.
	ReadyCondition = "Ready"
)

// Reasons that are used across different objects.
const (
	// ReadyReason indicates that the resource is ready for use.
	ReadyReason = "Ready"

	// NotReadyReason indicates that the resource is not ready for use.
	NotReadyReason = "NotReady"

	// ReconcilePendingReason indicates that the controller is waiting for resources to be reconciled.
	ReconcilePendingReason = "ReconcilePending"
)
