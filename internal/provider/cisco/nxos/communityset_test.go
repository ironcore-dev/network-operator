// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	cs := &CommunityList{}
	cs.Name = "TEST"
	cs.Mode = "regex"
	cs.EntItems.EntryList.Set(&communityEntry{
		Order:  10,
		Action: ActionPermit,
		Regex:  "50000:[0-9][0-9]",
	})
	Register("communityset", cs)

	ecs := &ExtCommunityList{}
	ecs.Name = "TEST-EXT"
	ecs.Mode = "regex"
	ecs.EntItems.EntryList.Set(&extCommunityEntry{
		Order:  15,
		Action: ActionPermit,
		Regex:  "65200:[0-9][0-9]",
	})
	Register("extcommunityset", ecs)
}
