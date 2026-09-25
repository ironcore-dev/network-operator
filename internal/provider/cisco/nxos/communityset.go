// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.CommunitySetProvider = (*Provider)(nil)

// CommunityList represents a named BGP community-list on NX-OS. Type selects the
// standard (rtregcom-items) or extended (rtextcom-items) container in the XPath.
type CommunityList struct {
	Type     v1alpha1.CommunitySetType `json:"-"`
	Name     string                    `json:"name"`
	Mode     string                    `json:"mode"`
	EntItems communityEntItems         `json:"ent-items"`
}

func (*CommunityList) IsListItem() {}

func (c *CommunityList) XPath() string {
	if c.Type == v1alpha1.CommunitySetTypeExtended {
		return "System/rpm-items/rtextcom-items/Rule-list[name=" + c.Name + "]"
	}
	return "System/rpm-items/rtregcom-items/Rule-list[name=" + c.Name + "]"
}

type communityEntItems struct {
	EntryList gnmiext.List[int32, *communityEntry] `json:"Entry-list"`
}

type communityEntry struct {
	Order  int32  `json:"order"`
	Action Action `json:"action"`
	Regex  string `json:"regex"`
}

func (e *communityEntry) Key() int32 { return e.Order }

func communityList(req *provider.CommunitySetRequest) *CommunityList {
	cl := &CommunityList{
		Type: req.CommunitySet.Spec.Type,
		Name: req.CommunitySet.Spec.Name,
		Mode: "regex",
	}
	for _, m := range req.CommunitySet.Spec.Members {
		cl.EntItems.EntryList.Set(&communityEntry{
			Order:  m.Sequence,
			Action: ActionPermit,
			Regex:  m.Regex,
		})
	}
	return cl
}

func (p *Provider) EnsureCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	return p.client.Update(ctx, communityList(req))
}

func (p *Provider) DeleteCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	return p.client.Delete(ctx, &CommunityList{
		Type: req.CommunitySet.Spec.Type,
		Name: req.CommunitySet.Spec.Name,
	})
}
