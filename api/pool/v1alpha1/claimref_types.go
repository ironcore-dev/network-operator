// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "k8s.io/apimachinery/pkg/types"

// ClaimRef identifies the Claim bound to an allocation object.
// +structType=atomic
type ClaimRef struct {
	// Name is the name of the Claim.
	// +required
	Name string `json:"name"`

	// UID is the UID of the Claim. When both name and UID match, the
	// allocation is considered fully bound. When the name matches but the
	// UID is stale or empty (e.g. after the original Claim was deleted and
	// recreated), the claim controller will only rebind if the allocation
	// carries the 'pool.networking.metal.ironcore.dev/allow-binding' annotation.
	// +required
	UID types.UID `json:"uid"`
}
