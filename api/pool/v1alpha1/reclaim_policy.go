// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ReclaimPolicy defines how allocations are handled on claim deletion.
// +kubebuilder:validation:Enum=Recycle;Retain
type ReclaimPolicy string

const (
	ReclaimPolicyRecycle ReclaimPolicy = "Recycle"
	ReclaimPolicyRetain  ReclaimPolicy = "Retain"
)
