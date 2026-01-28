// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

// AdminSt represents the administrative state of a component ("enabled" or "disabled").
type AdminSt string

const (
	AdminStEnabled  AdminSt = "enabled"
	AdminStDisabled AdminSt = "disabled"
)

// AdminSt2 represents the administrative state of a component ("up" or "down").
type AdminSt2 string

const (
	AdminStUp   AdminSt2 = "up"
	AdminStDown AdminSt2 = "down"
)

// AdminSt3 represents the administrative state of a component ("off" or "on").
type AdminSt3 string

const (
	AdminStOff AdminSt3 = "off"
	AdminStOn  AdminSt3 = "on"
)

// AdminSt4 represents the administrative state of a component ("enable" or "disable").
type AdminSt4 string

const (
	AdminStEnable  AdminSt4 = "enable"
	AdminStDisable AdminSt4 = "disable"
)

// OperSt represents the operational state of a component.
type OperSt string

const (
	OperStUp      OperSt = "up"
	OperStDown    OperSt = "down"
	OperStUnknown OperSt = "unknown"
	OperStLinkUp  OperSt = "link-up"
)
