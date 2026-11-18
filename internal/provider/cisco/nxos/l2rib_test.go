// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("l2rib", &HMMFwdEvpn{
		AdminSt:    AdminStEnabled,
		DdHmmoves:  10,
		DdHnsecs:   300,
		Dhrtimeout: 60,
		Dhrcount:   3,
	})
}
