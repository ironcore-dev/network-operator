// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.PrefixSetProvider = (*Provider)(nil)

func (p *Provider) EnsurePrefixSet(ctx context.Context, req *provider.PrefixSetRequest) error {
	spec := req.PrefixSet.Spec

	ps := &PrefixSetElement{
		Name: spec.Name,
		Config: &PrefixSetConfig{
			Name: spec.Name,
		},
		Prefixes: &PrefixSetPrefixes{},
	}

	for _, entry := range spec.Entries {
		mlr := MaskLengthRangeExact
		if entry.MaskLengthRange != nil {
			mlr = entry.MaskLengthRange.String()
		}
		ps.Prefixes.Prefix.Set(&PrefixSetPrefix{
			IPPrefix:     entry.Prefix.String(),
			MaskLenRange: mlr,
			Config: &PrefixSetPrefixConfig{
				IPPrefix:     entry.Prefix.String(),
				MaskLenRange: mlr,
			},
		})
	}

	return p.client.Update(ctx, ps)
}

func (p *Provider) DeletePrefixSet(ctx context.Context, req *provider.PrefixSetRequest) error {
	ps := &PrefixSetElement{Name: req.PrefixSet.Spec.Name}
	return p.client.Delete(ctx, ps)
}

// Compile-time assertion.
var _ gnmiext.DataElement = (*PrefixSetElement)(nil)

// PrefixSetElement targets a prefix-set entry.
type PrefixSetElement struct {
	Name     string             `json:"-"`
	Config   *PrefixSetConfig   `json:"config,omitempty"`
	Prefixes *PrefixSetPrefixes `json:"prefixes,omitempty"`
}

func (ps *PrefixSetElement) XPath() string {
	return fmt.Sprintf("openconfig-routing-policy:routing-policy/defined-sets/prefix-sets/prefix-set[name=%s]", ps.Name)
}

// PrefixSetConfig holds the prefix-set config.
type PrefixSetConfig struct {
	Name string `json:"name"`
}

// PrefixSetPrefixes holds the prefix list.
type PrefixSetPrefixes struct {
	Prefix gnmiext.List[string, *PrefixSetPrefix] `json:"prefix,omitempty"`
}

// PrefixSetPrefix represents a single prefix entry.
type PrefixSetPrefix struct {
	IPPrefix     string                 `json:"ip-prefix"`
	MaskLenRange string                 `json:"masklength-range"`
	Config       *PrefixSetPrefixConfig `json:"config,omitempty"`
}

func (p *PrefixSetPrefix) Key() string { return p.IPPrefix + "/" + p.MaskLenRange }

// PrefixSetPrefixConfig holds prefix config.
type PrefixSetPrefixConfig struct {
	IPPrefix     string `json:"ip-prefix"`
	MaskLenRange string `json:"masklength-range"`
}
