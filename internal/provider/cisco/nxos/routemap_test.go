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
}
