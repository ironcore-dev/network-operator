// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

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
}
