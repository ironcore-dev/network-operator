// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

// must panics if err is non-nil, otherwise returns v.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
