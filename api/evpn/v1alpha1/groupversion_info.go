// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "evpn.networking.metal.ironcore.dev", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

const (
	// FinalizerName is the identifier used by evpn controllers to perform cleanup before a resource is deleted.
	FinalizerName = "evpn.networking.metal.ironcore.dev/finalizer"

	// FabricLabel is applied to all sub-resources created by a Fabric controller,
	// with the value set to the owning Fabric's name.
	FabricLabel = "evpn.networking.metal.ironcore.dev/fabric"

	// NetworkLabel identifies the Network associated with an EVPN resource.
	NetworkLabel = "evpn.networking.metal.ironcore.dev/network"

	// NetworkAttachmentLabel identifies the NetworkAttachment that realizes an EVPN resource.
	NetworkAttachmentLabel = "evpn.networking.metal.ironcore.dev/network-attachment"
)

// Fabric condition types.
const (
	// UnderlayConvergedCondition reports whether all underlay IGP adjacencies are formed.
	UnderlayConvergedCondition = "UnderlayConverged"

	// OverlayConvergedCondition reports whether all overlay BGP sessions are established.
	OverlayConvergedCondition = "OverlayConverged"
)

// Fabric condition reasons.
const (
	// ConvergedReason indicates all child resources report Operational=True.
	ConvergedReason = "Converged"

	// NotConvergedReason indicates one or more child resources are not yet operational.
	NotConvergedReason = "NotConverged"

	// NoResourcesReason indicates no child resources exist yet for this condition.
	NoResourcesReason = "NoResources"

	// NoMatchingInterfacesReason indicates that an Interface selector matched no Interfaces.
	NoMatchingInterfacesReason = "NoMatchingInterfaces"

	// InvalidInterfaceReason indicates that a selected Interface cannot realize the attachment.
	InvalidInterfaceReason = "InvalidInterface"

	// DeviceResolutionFailedReason indicates that selected Interfaces reference unresolved Devices.
	DeviceResolutionFailedReason = "DeviceResolutionFailed"

	// FabricResolutionFailedReason indicates that selected Devices do not resolve to exactly one Fabric.
	FabricResolutionFailedReason = "FabricResolutionFailed"

	// EncapsulationConflictReason indicates that attachments request the same encapsulation on a Device.
	EncapsulationConflictReason = "EncapsulationConflict"

	// NetworkNotFoundReason indicates that the referenced Network does not exist.
	NetworkNotFoundReason = "NetworkNotFound"

	// InvalidVNIReason indicates that an allocated VNI is invalid.
	InvalidVNIReason = "InvalidVNI"

	// UnsupportedASNumberReason indicates that the Fabric AS number cannot be used for route-target derivation.
	UnsupportedASNumberReason = "UnsupportedASNumber"

	// ResourceConflictReason indicates that an existing resource prevents attachment realization.
	ResourceConflictReason = "ResourceConflict"

	// StretchedNetworkNotSupportedReason indicates that a Network resolves to multiple Fabrics.
	StretchedNetworkNotSupportedReason = "StretchedNetworkNotSupported"
)
