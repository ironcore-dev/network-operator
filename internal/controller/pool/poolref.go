// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

// poolRefIndexKey is the field index key shared by all allocation controllers
// (Index, IPAddress, IPPrefix) to look up objects by their referenced pool name.
const poolRefIndexKey = ".spec.poolRef.name"
