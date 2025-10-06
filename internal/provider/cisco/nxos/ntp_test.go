// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	ntp := &NTP{AdminSt: AdminStEnabled, Logging: AdminStEnabled}
	ntp.ProvItems.NtpProviderList = []*NTPProvider{
		{
			KeyID:     0,
			MaxPoll:   6,
			MinPoll:   4,
			Name:      "de.pool.ntp.org",
			Preferred: true,
			ProvT:     "server",
			Vrf:       "management",
		},
		{
			KeyID:     0,
			MaxPoll:   6,
			MinPoll:   4,
			Name:      "pool.ntp.org",
			Preferred: false,
			ProvT:     "server",
		},
	}
	ntp.SrcIfItems.SrcIf = "mgmt0"
	Register("ntp", ntp)
}
