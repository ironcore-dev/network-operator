// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package conditions provides utilities for managing status conditions on Kubernetes API objects.
package conditions

import (
	"cmp"
	"slices"

	grpcstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// Getter defines methods that an API object should implement in order to
// use the conditions package for getting conditions.
type Getter interface {
	// GetConditions returns the list of conditions for an API object.
	GetConditions() []metav1.Condition
}

// Setter defines methods that an API object should implement in order to
// use the conditions package for setting conditions.
type Setter interface {
	Getter
	// SetConditions sets conditions for an API object.
	SetConditions([]metav1.Condition)
}

// Set adds or updates a condition on the target object.
// It returns true if the condition was changed, false otherwise.
func Set(target Setter, condition metav1.Condition) (changed bool) {
	if m, ok := target.(metav1.Object); ok && condition.ObservedGeneration == 0 {
		condition.ObservedGeneration = m.GetGeneration()
	}
	conditions := target.GetConditions()
	if changed = meta.SetStatusCondition(&conditions, condition); !changed {
		return
	}
	Sort(conditions)
	target.SetConditions(conditions)
	return
}

// Del removes a condition of the specified type from the target object.
// It returns true if the condition was removed, false otherwise.
func Del(target Setter, conditionType string) (changed bool) {
	conditions := target.GetConditions()
	if changed = meta.RemoveStatusCondition(&conditions, conditionType); !changed {
		return
	}
	Sort(conditions)
	target.SetConditions(conditions)
	return
}

// IsReady looks at the [v1alpha1.ReadyCondition] condition type and returns true
// if that condition is set to true and the observed generation matches the object's generation.
func IsReady(target Getter) bool {
	condition := GetTopLevelCondition(target)
	if condition == nil {
		return false
	}
	if m, ok := target.(metav1.Object); ok && condition.ObservedGeneration != m.GetGeneration() {
		return false
	}
	return condition.Status == metav1.ConditionTrue
}

// IsConfigured looks at the [v1alpha1.ConfiguredCondition] condition type and returns true
// if that condition is set to true and the observed generation matches the object's generation.
func IsConfigured(target Getter) bool {
	condition := meta.FindStatusCondition(target.GetConditions(), v1alpha1.ConfiguredCondition)
	if condition == nil {
		return false
	}
	if m, ok := target.(metav1.Object); ok && condition.ObservedGeneration != m.GetGeneration() {
		return false
	}
	return condition.Status == metav1.ConditionTrue
}

// GetTopLevelCondition finds and returns the top level condition (Ready Condition).
func GetTopLevelCondition(target Getter) *metav1.Condition {
	return meta.FindStatusCondition(target.GetConditions(), v1alpha1.ReadyCondition)
}

// InitializeConditions updates all conditions to Unknown if not set.
func InitializeConditions(target Setter, types ...string) (changed bool) {
	conditions := target.GetConditions()
	for _, t := range types {
		if meta.FindStatusCondition(conditions, t) == nil {
			changed = changed || meta.SetStatusCondition(&conditions, metav1.Condition{
				Type:    t,
				Status:  metav1.ConditionUnknown,
				Reason:  v1alpha1.ReconcilePendingReason,
				Message: "Reconciliation has not yet completed",
			})
		}
	}
	Sort(conditions)
	target.SetConditions(conditions)
	return
}

// RecomputeReady recomputes the Ready Condition based on all other conditions.
// It sets the Ready Condition to false if any other condition is not ready,
// or to true if all other conditions are ready.
func RecomputeReady(target Setter) (changed bool) {
	status := metav1.ConditionTrue
	reason := v1alpha1.ReadyReason
	message := "All conditions are ready"

	conditions := target.GetConditions()
	for _, condition := range conditions {
		if condition.Type != v1alpha1.ReadyCondition && condition.Status != metav1.ConditionTrue {
			status = metav1.ConditionFalse
			reason = v1alpha1.NotReadyReason
			message = "One or more conditions are not ready"
			break
		}
	}

	condition := metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	return Set(target, condition)
}

// Sort sorts the given conditions slice in place.
// The Ready condition is sorted to the top, followed by other conditions
// in alphabetical order of their type.
func Sort(conditions []metav1.Condition) {
	slices.SortStableFunc(conditions, func(i, j metav1.Condition) int {
		switch {
		case i.Type == v1alpha1.ReadyCondition && j.Type != v1alpha1.ReadyCondition:
			return -1
		case i.Type != v1alpha1.ReadyCondition && j.Type == v1alpha1.ReadyCondition:
			return 1
		default:
			return cmp.Compare(i.Type, j.Type)
		}
	})
}

// FromError creates a [v1alpha1.ConfiguredCondition] from the given error.
// If the error is nil, it returns a condition indicating success.
// If the error is a gRPC status error, it extracts the code and message
// to populate the condition's Reason and Message fields.
func FromError(err error) metav1.Condition {
	cond := metav1.Condition{
		Type:    v1alpha1.ConfiguredCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ConfiguredReason,
		Message: "Configured successfully",
	}
	if err != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = v1alpha1.ErrorReason
		cond.Message = err.Error()

		// If the error is a gRPC status error, extract the code and message
		if statusErr, ok := grpcstatus.FromError(err); ok {
			cond.Reason = statusErr.Code().String()
			cond.Message = statusErr.Message()
		}
	}
	return cond
}
