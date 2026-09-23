// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

func init() {
	e := &RouteMapEntry{}
	e.Order = 10
	e.Action = ActionPermit
	e.SrttItems.ItemItems.ItemList.Set(&ExtCommItem{Community: "route-target:as2-nn2:65137:107", Scope: RtExtComScopeTransitive})
	e.SregcommItems.NoCommAttr = AdminStDisabled
	e.SregcommItems.ItemItems.ItemList.Set(&CommItem{Community: "regular:as2-nn2:65137:107"})
	e.MrtdstItems.RsrtDstAttItems.RsRtDstAttList.Set(&RsRtDstAtt{TDn: "/System/rpm-items/pfxlistv4-items/RuleV4-list[name='PL-CLOUD07']"})

	rm := &RouteMap{}
	rm.Name = "RM-REDIST"
	rm.EntItems.EntryList.Set(e)
	Register("route_map", rm)

	prependEntry := &RouteMapEntry{}
	prependEntry.Order = 10
	prependEntry.Action = ActionPermit
	prependEntry.SetASPathPrependItems.AS = "65000"

	prependRM := &RouteMap{}
	prependRM.Name = "RM-ASPATH-PREPEND"
	prependRM.EntItems.EntryList.Set(prependEntry)
	Register("route_map_aspath_prepend", prependRM)

	lastASEntry := &RouteMapEntry{}
	lastASEntry.Order = 10
	lastASEntry.Action = ActionPermit
	lastASEntry.SetASPathLastASItems.LastAS = 10

	lastASRM := &RouteMap{}
	lastASRM.Name = "RM-ASPATH-LASTAS"
	lastASRM.EntItems.EntryList.Set(lastASEntry)
	Register("route_map_aspath_lastas", lastASRM)

	replaceEntry1 := &RouteMapEntry{}
	replaceEntry1.Order = 10
	replaceEntry1.Action = ActionPermit
	replaceEntry1.SetASPathReplaceItems.MatchPrivateAS = true
	replaceEntry1.SetASPathReplaceItems.ReplaceAsn = "65000"
	replaceEntry1.SetASPathReplaceItems.ReplaceType = "asn"

	replaceEntry2 := &RouteMapEntry{}
	replaceEntry2.Order = 20
	replaceEntry2.Action = ActionPermit
	replaceEntry2.SetASPathReplaceItems.MatchAsnList = "65001"
	replaceEntry2.SetASPathReplaceItems.MatchPrivateAS = false
	replaceEntry2.SetASPathReplaceItems.ReplaceAsn = "65100"
	replaceEntry2.SetASPathReplaceItems.ReplaceType = "asn"

	replaceRM := &RouteMap{}
	replaceRM.Name = "RM-ASPATH-REPLACE"
	replaceRM.EntItems.EntryList.Set(replaceEntry1)
	replaceRM.EntItems.EntryList.Set(replaceEntry2)
	Register("route_map_aspath_replace", replaceRM)

	setEntry := &RouteMapEntry{}
	setEntry.Order = 10
	setEntry.Action = ActionPermit
	setEntry.SetASPathItems.AsnList = "65000"

	setRM := &RouteMap{}
	setRM.Name = "RM-ASPATH-SET"
	setRM.EntItems.EntryList.Set(setEntry)
	Register("route_map_aspath_set", setRM)

	pfxV4Entry := &RouteMapEntry{}
	pfxV4Entry.Order = 10
	pfxV4Entry.Action = ActionPermit
	pfxV4Entry.SetPrefixSet("PL-DEVICE-V4", false)

	pfxV4RM := &RouteMap{}
	pfxV4RM.Name = "RM-PREFIXSET-V4"
	pfxV4RM.EntItems.EntryList.Set(pfxV4Entry)
	Register("route_map_prefixset_v4", pfxV4RM)

	pfxV6Entry := &RouteMapEntry{}
	pfxV6Entry.Order = 10
	pfxV6Entry.Action = ActionPermit
	pfxV6Entry.SetPrefixSet("PL-DEVICE-V6", true)

	pfxV6RM := &RouteMap{}
	pfxV6RM.Name = "RM-PREFIXSET-V6"
	pfxV6RM.EntItems.EntryList.Set(pfxV6Entry)
	Register("route_map_prefixset_v6", pfxV6RM)

	commAnyEntry := &RouteMapEntry{}
	commAnyEntry.Order = 10
	commAnyEntry.Action = ActionPermit
	commAnyEntry.SetCommunitySet("CS-BLUE", false)

	commAnyRM := &RouteMap{}
	commAnyRM.Name = "RM-COMMUNITYSET-ANY"
	commAnyRM.EntItems.EntryList.Set(commAnyEntry)
	Register("route_map_communityset_any", commAnyRM)

	commAllEntry := &RouteMapEntry{}
	commAllEntry.Order = 10
	commAllEntry.Action = ActionPermit
	commAllEntry.SetCommunitySet("CS-BLUE", true)

	commAllRM := &RouteMap{}
	commAllRM.Name = "RM-COMMUNITYSET-ALL"
	commAllRM.EntItems.EntryList.Set(commAllEntry)
	Register("route_map_communityset_all", commAllRM)

	extCommAnyEntry := &RouteMapEntry{}
	extCommAnyEntry.Order = 10
	extCommAnyEntry.Action = ActionPermit
	extCommAnyEntry.SetExtCommunitySet("RT-BLUE", false)

	extCommAnyRM := &RouteMap{}
	extCommAnyRM.Name = "RM-EXTCOMMUNITYSET-ANY"
	extCommAnyRM.EntItems.EntryList.Set(extCommAnyEntry)
	Register("route_map_extcommunityset_any", extCommAnyRM)

	extCommAllEntry := &RouteMapEntry{}
	extCommAllEntry.Order = 10
	extCommAllEntry.Action = ActionPermit
	extCommAllEntry.SetExtCommunitySet("RT-BLUE", true)

	extCommAllRM := &RouteMap{}
	extCommAllRM.Name = "RM-EXTCOMMUNITYSET-ALL"
	extCommAllRM.EntItems.EntryList.Set(extCommAllEntry)
	Register("route_map_extcommunityset_all", extCommAllRM)

	comboEntry := &RouteMapEntry{}
	comboEntry.Order = 10
	comboEntry.Action = ActionPermit
	comboEntry.SetPrefixSet("PL-DEVICE-V4", false)
	comboEntry.SetCommunitySet("CS-BLUE", false)
	comboEntry.SetExtCommunitySet("RT-BLUE", true)

	comboRM := &RouteMap{}
	comboRM.Name = "RM-COMBINED-MATCH"
	comboRM.EntItems.EntryList.Set(comboEntry)
	Register("route_map_combined_match", comboRM)

	commAdditiveEntry := &RouteMapEntry{}
	commAdditiveEntry.Order = 10
	commAdditiveEntry.Action = ActionPermit
	if err := commAdditiveEntry.SetCommunities([]string{"65000:10", "65000:20"}, v1alpha1.CommunityOptionsAdd); err != nil {
		panic(err)
	}

	commAdditiveRM := &RouteMap{}
	commAdditiveRM.Name = "RM-COMMUNITY-ADDITIVE"
	commAdditiveRM.EntItems.EntryList.Set(commAdditiveEntry)
	Register("route_map_community_additive", commAdditiveRM)

	extCommAdditiveEntry := &RouteMapEntry{}
	extCommAdditiveEntry.Order = 10
	extCommAdditiveEntry.Action = ActionPermit
	if err := extCommAdditiveEntry.SetExtCommunities([]string{"65000:10", "65000:20"}, v1alpha1.CommunityOptionsAdd); err != nil {
		panic(err)
	}

	extCommAdditiveRM := &RouteMap{}
	extCommAdditiveRM.Name = "RM-EXTCOMMUNITY-ADDITIVE"
	extCommAdditiveRM.EntItems.EntryList.Set(extCommAdditiveEntry)
	Register("route_map_extcommunity_additive", extCommAdditiveRM)
}
