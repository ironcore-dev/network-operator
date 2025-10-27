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

// DeviceLabel is a label applied to any Network API object to indicate the device
// it is associated with. This label is used by controllers to filter and manage resources
// based on the device they are intended for.
const DeviceLabel = "networking.cloud.sap/device-name"

// DeviceKind represents the Kind of Device.
const DeviceKind = "Device"

// AggregateLabel is a label applied to member interfaces to indicate
// the name of the aggregate interface they belong to.
const AggregateLabel = "networking.cloud.sap/aggregate-name"

// Condition types that are used across different objects.
const (
	// ReadyCondition is the top-level status condition that reports if an object is ready.
	// This condition indicates whether the resource is ready to be used and will
	// be calculated by the controller based on child conditions, if present.
	ReadyCondition = "Ready"

	// ConfiguredCondition indicates whether the resource has been successfully configured.
	// This condition indicates whether the desired configuration has been applied to the device
	// (i.e., all necessary API calls succeeded).
	ConfiguredCondition = "Configured"

	// OperationalCondition indicates whether the resource is operational.
	// This condition indicates whether the resource is in a state that allows it to function as
	// intended (e.g., a interface is up). It corresponds to the "oper-status" commonly found
	// in the OpenConfig models (or "operSt" on Cisco).
	OperationalCondition = "Operational"
)

// Reasons that are used across different objects.
const (
	// ReadyReason indicates that the resource is ready for use.
	ReadyReason = "Ready"

	// NotReadyReason indicates that the resource is not ready for use.
	NotReadyReason = "NotReady"

	// ReconcilePendingReason indicates that the controller is waiting for resources to be reconciled.
	ReconcilePendingReason = "ReconcilePending"

	// NotImplementedReason indicates that the provider does not implement the required functionality
	// to support the resource.
	NotImplementedReason = "NotImplemented"

	// ProvisioningReason indicates that the resource is being provisioned.
	ProvisioningReason = "Provisioning"

	// ConfiguredReason indicates that the resource has been successfully configured.
	ConfiguredReason = "Configured"

	// OperationalReason indicates that the resource is operational.
	OperationalReason = "Operational"

	// DegradedReason indicates that the resource is in a degraded state.
	DegradedReason = "Degraded"

	// ErrorReason indicates that an error occurred while reconciling the resource.
	ErrorReason = "Error"

	// WaitingForDependenciesReason indicates that the resource is waiting for its dependencies to be ready.
	WaitingForDependenciesReason = "WaitingForDependencies"
)

// Reasons that are specific to [Interface] objects.
const (
	// InterfaceNotFoundReason indicates that a referenced interface was not found.
	InterfaceNotFoundReason = "InterfaceNotFound"

	// InvalidInterfaceTypeReason indicates that a referenced interface is not of the expected type.
	InvalidInterfaceTypeReason = "InvalidInterfaceType"

	// CrossDeviceReferenceReason indicates that a referenced interface belongs to a different device.
	CrossDeviceReferenceReason = "CrossDeviceReference"

	// MemberInterfaceAlreadyInUseReason indicates that a member interface is already part of another aggregate.
	MemberInterfaceAlreadyInUseReason = "MemberInterfaceAlreadyInUse"
)
