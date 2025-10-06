// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "time"

func init() {
	var vty VTY
	vty.ExecTmeoutItems.Timeout = int(time.Hour.Minutes())
	vty.SsLmtItems.SesLmt = 8

	Register("console", &Console{Timeout: int(time.Hour.Minutes())})
	Register("vty_acl", &VTYAccessClass{Name: "TEST-ACL"})
	Register("vty", &vty)
}
