// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package paused implements helper functions for managing the Paused condition on API objects.
package paused

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// Object combines [client.Object] with [conditions.Setter].
type Object interface {
	client.Object
	conditions.Setter
}

// EnsureCondition computes and patches the "Paused" condition on the object.
// It returns whether the object is paused, whether the caller should requeue,
// and any error encountered while patching.
func EnsureCondition(ctx context.Context, c client.Client, device *v1alpha1.Device, obj Object) (isPaused, requeue bool, err error) {
	log := ctrl.LoggerFrom(ctx)

	oldCondition := conditions.Get(obj, v1alpha1.PausedCondition)
	newCondition := computeCondition(device, obj)

	isPaused = newCondition.Status == metav1.ConditionTrue
	statusChanged := oldCondition == nil || oldCondition.Status != newCondition.Status

	switch {
	case statusChanged && isPaused:
		log.Info("Pausing reconciliation for this object", "reason", newCondition.Message)
	case statusChanged && !isPaused && oldCondition != nil:
		log.Info("Unpausing reconciliation for this object")
	case !statusChanged && isPaused:
		log.V(3).Info("Reconciliation is paused for this object", "reason", newCondition.Message)
	}

	// Set Ready=Unknown while paused: the operator is no longer actively
	// verifying the resource, so its state cannot be determined.
	if isPaused {
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.PausedReason,
			Message: "Reconciliation is paused",
		})
	}

	// Only do a standalone status patch when pausing. When not paused,
	// the condition is set in-memory and will be persisted by the normal
	// reconciliation status update, avoiding an unnecessary extra reconcile.
	orig := obj.DeepCopyObject().(client.Object)
	if changed := conditions.Set(obj, newCondition); !changed || !isPaused {
		return isPaused, false, nil
	}

	if err := c.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
		return isPaused, false, err
	}

	return isPaused, true, nil
}

// computeCondition builds the Paused condition. A resource is paused when
// any of the following apply (in priority order):
//  1. device.spec.paused is true
//  2. device.status.phase is not Running (child resources only)
//  3. device's Reachable condition is not true (child resources only)
//  4. the object carries [v1alpha1.PausedAnnotation]
func computeCondition(device *v1alpha1.Device, obj Object) metav1.Condition {
	condition := metav1.Condition{
		Type:               v1alpha1.PausedCondition,
		Status:             metav1.ConditionFalse,
		Reason:             v1alpha1.NotPausedReason,
		ObservedGeneration: obj.GetGeneration(),
	}

	if device != nil {
		if device.Spec.Paused {
			condition.Status = metav1.ConditionTrue
			condition.Reason = v1alpha1.PausedReason
			condition.Message = "Device spec.paused is set to true"
			return condition
		}
		// Phase and reachability checks only apply to child resources
		// (device != obj). The device itself must not pause due to its
		// own phase or reachability — it needs to keep reconciling to
		// reach Running and to set the Reachable condition.
		if device != obj {
			if device.Status.Phase != v1alpha1.DevicePhaseRunning {
				condition.Status = metav1.ConditionTrue
				condition.Reason = v1alpha1.PausedReason
				condition.Message = "Device is not in phase Running"
				return condition
			}
			if cond := conditions.Get(device, v1alpha1.ReachableCondition); cond != nil && cond.Status != metav1.ConditionTrue {
				condition.Status = metav1.ConditionTrue
				condition.Reason = v1alpha1.PausedReason
				condition.Message = "Device is not reachable: " + cond.Message
				return condition
			}
		}
	}

	if _, ok := obj.GetAnnotations()[v1alpha1.PausedAnnotation]; ok {
		condition.Status = metav1.ConditionTrue
		condition.Reason = v1alpha1.PausedReason
		condition.Message = fmt.Sprintf("%s has the %s annotation", obj.GetObjectKind().GroupVersionKind().Kind, v1alpha1.PausedAnnotation)
	}

	return condition
}

// DevicePausedChanged reports whether the device's effective pause state changed
// between the old and new object versions. The effective pause state is
// determined by [computeCondition].
func DevicePausedChanged(oldObj, newObj client.Object) bool {
	oldDevice := oldObj.(*v1alpha1.Device)
	newDevice := newObj.(*v1alpha1.Device)
	if oldDevice.Spec.Paused != newDevice.Spec.Paused {
		return true
	}
	oldPhaseRunning := oldDevice.Status.Phase == v1alpha1.DevicePhaseRunning
	newPhaseRunning := newDevice.Status.Phase == v1alpha1.DevicePhaseRunning
	if oldPhaseRunning != newPhaseRunning {
		return true
	}
	oldReachable := conditions.Get(oldDevice, v1alpha1.ReachableCondition)
	newReachable := conditions.Get(newDevice, v1alpha1.ReachableCondition)
	oldIsReachable := oldReachable == nil || oldReachable.Status == metav1.ConditionTrue
	newIsReachable := newReachable == nil || newReachable.Status == metav1.ConditionTrue
	return oldIsReachable != newIsReachable
}
