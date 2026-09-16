// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ provider.CommunitySetProvider    = (*Provider)(nil)
	_ provider.ExtCommunitySetProvider = (*Provider)(nil)
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*CommunitySetElement)(nil)
	_ gnmiext.DataElement = (*ExtCommunitySetElement)(nil)
)

func (p *Provider) EnsureCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	spec := req.CommunitySet.Spec

	cs := &CommunitySetElement{
		Name: spec.Name,
		Config: &CommunitySetConfig{
			Name: spec.Name,
		},
	}
	for _, m := range spec.Members {
		cs.Config.Members = append(cs.Config.Members, m.Regex)
	}

	return p.client.Update(ctx, cs)
}

func (p *Provider) DeleteCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	cs := &CommunitySetElement{Name: req.CommunitySet.Spec.Name}
	return p.client.Delete(ctx, cs)
}

func (p *Provider) EnsureExtCommunitySet(ctx context.Context, req *provider.ExtCommunitySetRequest) error {
	spec := req.ExtCommunitySet.Spec

	cs := &ExtCommunitySetElement{
		Name: spec.Name,
		Config: &ExtCommunitySetConfig{
			Name: spec.Name,
		},
	}
	for _, m := range spec.Members {
		cs.Config.Members = append(cs.Config.Members, m.Regex)
	}

	return p.client.Update(ctx, cs)
}

func (p *Provider) DeleteExtCommunitySet(ctx context.Context, req *provider.ExtCommunitySetRequest) error {
	cs := &ExtCommunitySetElement{Name: req.ExtCommunitySet.Spec.Name}
	return p.client.Delete(ctx, cs)
}

// CommunitySetElement targets a community-set entry under bgp-defined-sets.
type CommunitySetElement struct {
	Name   string              `json:"-"`
	Config *CommunitySetConfig `json:"config,omitempty"`
}

func (cs *CommunitySetElement) XPath() string {
	return fmt.Sprintf("openconfig-routing-policy:routing-policy/defined-sets/openconfig-bgp-policy:bgp-defined-sets/community-sets/community-set[community-set-name=%s]", cs.Name)
}

// CommunitySetConfig holds the community-set config.
type CommunitySetConfig struct {
	Name    string   `json:"community-set-name"`
	Members []string `json:"community-member"`
}

// ExtCommunitySetElement targets an ext-community-set entry under bgp-defined-sets.
type ExtCommunitySetElement struct {
	Name   string                 `json:"-"`
	Config *ExtCommunitySetConfig `json:"config,omitempty"`
}

func (cs *ExtCommunitySetElement) XPath() string {
	return fmt.Sprintf("openconfig-routing-policy:routing-policy/defined-sets/openconfig-bgp-policy:bgp-defined-sets/ext-community-sets/ext-community-set[ext-community-set-name=%s]", cs.Name)
}

// ExtCommunitySetConfig holds the ext-community-set config.
type ExtCommunitySetConfig struct {
	Name    string   `json:"ext-community-set-name"`
	Members []string `json:"ext-community-member"`
}
