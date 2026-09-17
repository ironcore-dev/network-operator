// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ provider.CommunitySetProvider    = (*Provider)(nil)
	_ provider.ExtCommunitySetProvider = (*Provider)(nil)
)

// CommunityList represents a named BGP standard community-list on NX-OS.
type CommunityList struct {
	Name     string            `json:"name"`
	Mode     string            `json:"mode"`
	EntItems communityEntItems `json:"ent-items"`
}

func (*CommunityList) IsListItem() {}

func (c *CommunityList) XPath() string {
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

// ExtCommunityList represents a named BGP extended community-list on NX-OS.
type ExtCommunityList struct {
	Name     string               `json:"name"`
	Mode     string               `json:"mode"`
	EntItems extCommunityEntItems `json:"ent-items"`
}

func (*ExtCommunityList) IsListItem() {}

func (c *ExtCommunityList) XPath() string {
	return "System/rpm-items/rtextcom-items/Rule-list[name=" + c.Name + "]"
}

type extCommunityEntItems struct {
	EntryList gnmiext.List[int32, *extCommunityEntry] `json:"Entry-list"`
}

type extCommunityEntry struct {
	Order  int32  `json:"order"`
	Action Action `json:"action"`
	Regex  string `json:"regex"`
}

func (e *extCommunityEntry) Key() int32 { return e.Order }

func (p *Provider) EnsureCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	cl := new(CommunityList)
	cl.Name = req.CommunitySet.Spec.Name
	cl.Mode = "regex"
	for _, m := range req.CommunitySet.Spec.Members {
		cl.EntItems.EntryList.Set(&communityEntry{
			Order:  m.Sequence,
			Action: ActionPermit,
			Regex:  m.Regex,
		})
	}
	return p.client.Update(ctx, cl)
}

func (p *Provider) DeleteCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	cl := new(CommunityList)
	cl.Name = req.CommunitySet.Spec.Name
	return p.client.Delete(ctx, cl)
}

func (p *Provider) EnsureExtCommunitySet(ctx context.Context, req *provider.ExtCommunitySetRequest) error {
	cl := new(ExtCommunityList)
	cl.Name = req.ExtCommunitySet.Spec.Name
	cl.Mode = "regex"
	for _, m := range req.ExtCommunitySet.Spec.Members {
		cl.EntItems.EntryList.Set(&extCommunityEntry{
			Order:  m.Sequence,
			Action: ActionPermit,
			Regex:  m.Regex,
		})
	}
	return p.client.Update(ctx, cl)
}

func (p *Provider) DeleteExtCommunitySet(ctx context.Context, req *provider.ExtCommunitySetRequest) error {
	cl := new(ExtCommunityList)
	cl.Name = req.ExtCommunitySet.Spec.Name
	return p.client.Delete(ctx, cl)
}
