// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import "time"

const (
	// DefaultTimeout is used for standard resource reconciliation
	DefaultTimeout = 30 * time.Second

	// LongTimeout is used for operations that may take longer (deployments, pod starts)
	LongTimeout = 2 * time.Minute

	// VeryLongTimeout is used for end-to-end scenarios with multiple dependencies
	VeryLongTimeout = 5 * time.Minute
)
