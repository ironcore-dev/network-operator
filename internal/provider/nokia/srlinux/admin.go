// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

// AdminSt is the SR Linux admin-state leaf type. The device default is Enable,
// so it is always written explicitly to keep the Get-and-Check diff stable.
type AdminSt string

const (
	AdminStEnable  AdminSt = "enable"
	AdminStDisable AdminSt = "disable"
)
