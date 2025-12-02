// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*RouteMap)(nil)

type RouteMap struct {
	Name     string `json:"name"`
	EntItems struct {
		EntryList gnmiext.List[int32, *RouteMapEntry] `json:"Entry-list,omitzero"`
	} `json:"ent-items,omitzero"`
}

func (*RouteMap) IsListItem() {}

func (rm *RouteMap) XPath() string {
	return "System/rpm-items/rtmap-items/Rule-list[name=" + rm.Name + "]"
}

type RouteMapEntry struct {
	Action    Action `json:"action"`
	Order     int32  `json:"order"`
	SrttItems struct {
		ItemItems struct {
			ItemList gnmiext.List[ExtCommItem, *ExtCommItem] `json:"Item-list,omitzero"`
		} `json:"item-items,omitzero"`
	} `json:"srtt-items,omitzero"`
	SregcommItems struct {
		NoCommAttr AdminSt `json:"noCommAttr"`
		ItemItems  struct {
			ItemList gnmiext.List[string, *CommItem] `json:"Item-list,omitzero"`
		} `json:"item-items,omitzero"`
	} `json:"sregcomm-items,omitzero"`
	MrtdstItems struct {
		RsrtDstAttItems struct {
			RsRtDstAttList gnmiext.List[string, *RsRtDstAtt] `json:"RsRtDstAtt-list,omitzero"`
		} `json:"rsrtDstAtt-items,omitzero"`
	} `json:"mrtdst-items,omitzero"`
}

func (e *RouteMapEntry) Key() int32 { return e.Order }

func (e *RouteMapEntry) SetCommunities(communities []string) error {
	for _, comm := range communities {
		c, err := Community(comm)
		if err != nil {
			return err
		}
		e.SregcommItems.NoCommAttr = AdminStDisabled
		e.SregcommItems.ItemItems.ItemList.Set(&CommItem{Community: c})
	}
	return nil
}

func (e *RouteMapEntry) SetExtCommunities(communities []string) error {
	for _, comm := range communities {
		c, err := RouteTarget(comm)
		if err != nil {
			return err
		}
		e.SrttItems.ItemItems.ItemList.Set(&ExtCommItem{Community: c, Scope: RtExtComScopeTransitive})
	}
	return nil
}

func (e *RouteMapEntry) SetPrefixSet(ps *v1alpha1.PrefixSet) {
	tdn := "/System/rpm-items/pfxlistv4-items/RuleV4-list[name='" + ps.Name + "']"
	if ps.Is6() {
		tdn = "/System/rpm-items/pfxlistv6-items/RuleV6-list[name='" + ps.Name + "']"
	}
	e.MrtdstItems.RsrtDstAttItems.RsRtDstAttList.Set(&RsRtDstAtt{TDn: tdn})
}

type RsRtDstAtt struct {
	TDn string `json:"tDn"`
}

func (r *RsRtDstAtt) Key() string { return r.TDn }

type CommItem struct {
	Community string `json:"community"`
}

func (c *CommItem) Key() string { return c.Community }

type ExtCommItem struct {
	Community string        `json:"community"`
	Scope     RtExtComScope `json:"scope"`
}

func (c *ExtCommItem) Key() ExtCommItem { return *c }

type RtExtComScope string

const (
	RtExtComScopeTransitive    RtExtComScope = "transitive"
	RtExtComScopeNonTransitive RtExtComScope = "non-transitive"
)
