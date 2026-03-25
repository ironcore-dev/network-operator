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

	// ErrAllocationInconsistent is returned when a claim carries an allocation in its
	// status that is not reflected in the pool's allocations, indicating external
	// modification or a partial write that requires manual intervention.
	ErrAllocationInconsistent = errors.New("claim allocation is inconsistent with pool")

	// ErrPreferredValueUnavailable is returned by AllocatePreferred when the requested
	// value is outside the pool's configured ranges/prefixes or is already taken.
	ErrPreferredValueUnavailable = errors.New("preferred value unavailable")

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

	// AllocationFailedReason indicates that allocation could not be completed.
	AllocationFailedReason = "AllocationFailed"

	// PreferredValueUnavailableReason indicates that the requested preferred value is not available.
	PreferredValueUnavailableReason = "PreferredValueUnavailable"
)

// Annotation keys
const (
	// PreferredValueAnnotation is an optional annotation on a Claim that requests a specific
	// allocation value. The format depends on the pool type:
	//   - IndexPool: decimal uint64, e.g. "64512"
	//   - IPAddressPool: IP address string, e.g. "10.0.0.42"
	//   - IPPrefixPool: CIDR string, e.g. "192.168.5.0/24"
	// If the value is unavailable the claim enters a terminal error state with reason
	// PreferredValueUnavailable. Remove the annotation to fall back to normal allocation.
	PreferredValueAnnotation = "pool.networking.metal.ironcore.dev/preferred-value"
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
