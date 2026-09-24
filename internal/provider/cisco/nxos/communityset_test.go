// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

func init() {
	std := &CommunityList{Type: v1alpha1.CommunitySetTypeStandard}
	std.Name = "TEST"
	std.Mode = "regex"
	std.EntItems.EntryList.Set(&communityEntry{
		Order:  10,
		Action: ActionPermit,
		Regex:  "50000:[0-9][0-9]",
	})
	Register("communityset", std)

	ext := &CommunityList{Type: v1alpha1.CommunitySetTypeExtended}
	ext.Name = "TEST-EXT"
	ext.Mode = "regex"
	ext.EntItems.EntryList.Set(&communityEntry{
		Order:  15,
		Action: ActionPermit,
		Regex:  "65200:[0-9][0-9]",
	})
	Register("communityset_external", ext)
}
