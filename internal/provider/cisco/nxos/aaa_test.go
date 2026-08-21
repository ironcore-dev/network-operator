// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("aaa_tacacs_server", &TacacsPlusProvider{
		Name:    "10.1.1.1",
		Port:    49,
		KeyEnc:  "7",
		Timeout: 5,
	})

	tacacsGroup := &TacacsPlusProviderGroup{
		Name:  "TACACS-SERVERS",
		Vrf:   "management",
		SrcIf: NewOption("mgmt0"),
	}
	tacacsGroup.ProviderRefItems.ProviderRefList.Set(&TacacsPlusProviderRef{Name: "10.1.1.1"})
	Register("aaa_tacacs_group", tacacsGroup)

	Register("aaa_defaultauth", &AAADefaultAuth{
		Realm:         AAARealmTacacs,
		ProviderGroup: "TACACS-SERVERS",
		Fallback:      AAAValueYes,
		Local:         AAAValueYes,
		ErrEn:         true,
	})

	Register("aaa_consoleauth", &AAAConsoleAuth{
		Realm:         AAARealmTacacs,
		ProviderGroup: "TACACS-SERVERS",
		Fallback:      AAAValueYes,
		Local:         AAAValueYes,
	})

	Register("aaa_author", &AAADefaultAuthor{
		CmdType:       "config",
		ProviderGroup: "TACACS-SERVERS",
	})

	Register("aaa_acct", &AAADefaultAcc{
		Realm:         AAARealmTacacs,
		ProviderGroup: "TACACS-SERVERS",
	})
}
