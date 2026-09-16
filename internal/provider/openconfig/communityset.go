// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.CommunitySetProvider = (*Provider)(nil)

// Compile-time assertions.
var _ gnmiext.DataElement = (*CommunitySetElement)(nil)

func (p *Provider) EnsureCommunitySet(ctx context.Context, req *provider.CommunitySetRequest) error {
	spec := req.CommunitySet.Spec

	if spec.Type == v1alpha1.CommunitySetTypeExtended {
		return p.ensureExtCommunitySet(ctx, &spec)
	}
	return p.ensureStdCommunitySet(ctx, &spec)
}

func (p *Provider) DeleteCommunitySet(ctx context.Context, req *provider.CommunitySetDeleteRequest) error {
	spec := req.CommunitySet.Spec
	var e gnmiext.DataElement
	if spec.Type == v1alpha1.CommunitySetTypeExtended {
		e = &ExtCommunitySetElement{Name: spec.Name}
	} else {
		e = &CommunitySetElement{Name: spec.Name}
	}
	return p.client.Delete(ctx, e)
}

func (p *Provider) ensureStdCommunitySet(ctx context.Context, spec *v1alpha1.CommunitySetSpec) error {
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

func (p *Provider) ensureExtCommunitySet(ctx context.Context, spec *v1alpha1.CommunitySetSpec) error {
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

// CommunitySetElement targets a community-set entry under bgp-defined-sets.
type CommunitySetElement struct {
	Name   string              `json:"-"`
	Config *CommunitySetConfig `json:"config"`
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
	Config *ExtCommunitySetConfig `json:"config"`
}

func (cs *ExtCommunitySetElement) XPath() string {
	return fmt.Sprintf("openconfig-routing-policy:routing-policy/defined-sets/openconfig-bgp-policy:bgp-defined-sets/ext-community-sets/ext-community-set[ext-community-set-name=%s]", cs.Name)
}

// ExtCommunitySetConfig holds the ext-community-set config.
type ExtCommunitySetConfig struct {
	Name    string   `json:"ext-community-set-name"`
	Members []string `json:"ext-community-member"`
}
