// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "pool.networking.metal.ironcore.dev", Version: "v1alpha1"}

	// ErrPoolExhausted is returned by Allocate when no free value remains in the pool.
	ErrPoolExhausted = errors.New("pool is exhausted")

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

const (
	// FinalizerName is the identifier used by pool controllers to perform cleanup before a resource is deleted.
	FinalizerName = "pool.networking.metal.ironcore.dev/finalizer"
)

const (
	// AllowBindingAnnotation permits an allocation object whose claimRef name matches
	// a Claim but whose UID is stale (e.g. after the Claim was deleted and recreated)
	// to be rebound by updating the UID to the current Claim.
	AllowBindingAnnotation = "pool.networking.metal.ironcore.dev/allow-binding"
)

// Allocated condition — set on Claim; reports whether it has successfully reserved a resource.
const (
	// AllocatedCondition reports whether a Claim has successfully reserved a resource.
	AllocatedCondition = "Allocated"

	// AllocatedReason indicates that an allocation has been successfully reserved for a Claim.
	AllocatedReason = "Allocated"

	// PoolRefInvalidReason indicates that a Claim references an invalid or unsupported pool.
	PoolRefInvalidReason = "PoolRefInvalid"

	// PoolNotFoundReason indicates that a referenced pool resource does not exist.
	PoolNotFoundReason = "PoolNotFound"

	// PoolExhaustedReason indicates that a pool has no available allocations.
	PoolExhaustedReason = "PoolExhausted"

	// MultipleAllocationsReason indicates that more than one allocation object
	// is bound to the same claim.
	MultipleAllocationsReason = "MultipleAllocations"
)

// Available condition — set on pool types; reports whether the pool has free capacity.
const (
	// AvailableCondition reports whether the pool has free capacity for new claims.
	AvailableCondition = "Available"

	// HasCapacityReason indicates the pool has at least one free slot.
	HasCapacityReason = "HasCapacity"

	// ExhaustedReason indicates the pool has no free slots.
	ExhaustedReason = "Exhausted"
)

// Valid condition — set on allocation objects (Index, IPAddress, IPPrefix).
const (
	// ValidCondition reports whether an allocation object's value is valid within the referenced pool.
	ValidCondition = "Valid"

	// ValueInRangeReason indicates the value falls within the pool's configured ranges/prefixes.
	ValueInRangeReason = "ValueInRange"

	// ValueOutOfRangeReason indicates the value falls outside the pool's configured ranges/prefixes.
	ValueOutOfRangeReason = "ValueOutOfRange"

	// PoolNotFoundForValidationReason indicates the referenced pool does not exist.
	PoolNotFoundForValidationReason = "PoolNotFound"
)
