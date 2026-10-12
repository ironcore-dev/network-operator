// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	sidod := DHCPRelayServerIDOverrideDisable(1)
	Register("dhcprelayserveridoverride", &sidod)
}
