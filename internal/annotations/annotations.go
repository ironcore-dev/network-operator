// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package annotations implements annotation helper functions.
package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IsPaused returns true if the Device is paused or the object has the [v1alpha1.PausedAnnotation].
func IsPaused(device *v1alpha1.Device, obj metav1.Object) bool {
	return (device.Spec.Paused != nil && *device.Spec.Paused) || HasPaused(obj)
}

// HasPaused returns true if the object has the [v1alpha1.PausedAnnotation].
func HasPaused(obj metav1.Object) bool {
	return Has(obj, v1alpha1.PausedAnnotation)
}

// Has returns true if the object has the specified annotation.
func Has(obj metav1.Object, annotation string) bool {
	_, ok := obj.GetAnnotations()[annotation]
	return ok
}
