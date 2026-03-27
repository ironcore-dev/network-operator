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
	case statusChanged && !isPaused:
		log.Info("Unpausing reconciliation for this object")
	case !statusChanged && isPaused:
		log.V(4).Info("Reconciliation is paused for this object", "reason", newCondition.Message)
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

	if err = c.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
		return isPaused, false, err
	}

	return isPaused, true, nil
}

// computeCondition builds the Paused condition based on [v1alpha1.Device.Spec.Paused]
// and the presence of the [v1alpha1.PausedAnnotation] on the object.
func computeCondition(device *v1alpha1.Device, obj Object) metav1.Condition {
	condition := metav1.Condition{
		Type:               v1alpha1.PausedCondition,
		Status:             metav1.ConditionFalse,
		Reason:             v1alpha1.NotPausedReason,
		ObservedGeneration: obj.GetGeneration(),
	}

	if device != nil && device.Spec.Paused {
		condition.Status = metav1.ConditionTrue
		condition.Reason = v1alpha1.PausedReason
		condition.Message = "Device spec.paused is set to true"
		return condition
	}

	if _, ok := obj.GetAnnotations()[v1alpha1.PausedAnnotation]; ok {
		condition.Status = metav1.ConditionTrue
		condition.Reason = v1alpha1.PausedReason
		condition.Message = fmt.Sprintf("%s has the %s annotation", obj.GetObjectKind().GroupVersionKind().Kind, v1alpha1.PausedAnnotation)
	}

	return condition
}
