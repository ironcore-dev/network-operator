// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

func init() {
	ntp := &NTP{AdminSt: AdminStEnabled, Logging: AdminStEnabled}
	ntp.ProvItems.NtpProviderList = make(gnmiext.List[string, *NTPProvider])
	ntp.ProvItems.NtpProviderList.Set(&NTPProvider{
		KeyID:     0,
		MaxPoll:   6,
		MinPoll:   4,
		Name:      "de.pool.ntp.org",
		Preferred: true,
		ProvT:     "server",
		Vrf:       "management",
	})
	ntp.SrcIfItems.SrcIf = "mgmt0"
	Register("ntp", ntp)
}
