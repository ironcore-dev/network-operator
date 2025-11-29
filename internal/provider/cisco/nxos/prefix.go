// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*PrefixList)(nil)

type PrefixList struct {
	Name     string `json:"name"`
	EntItems struct {
		EntryList gnmiext.List[int32, *PrefixEntry] `json:"Entry-list"`
	} `json:"ent-items"`
	// Is6 indicates whether this is an IPv6 prefix list. This field is not serialized to JSON
	// and is only used internally to determine the correct XPath for the prefix list.
	Is6 bool `json:"-"`
}

func (*PrefixList) IsListItem() {}

func (p *PrefixList) XPath() string {
	if p.Is6 {
		return "System/rpm-items/pfxlistv6-items/RuleV6-list[name=" + p.Name + "]"
	}
	return "System/rpm-items/pfxlistv4-items/RuleV4-list[name=" + p.Name + "]"
}

type PrefixEntry struct {
	Action     Action   `json:"action"`
	Criteria   Criteria `json:"criteria"`
	FromPfxLen int8     `json:"fromPfxLen"`
	Order      int32    `json:"order"`
	Pfx        string   `json:"pfx"`
	ToPfxLen   int8     `json:"toPfxLen"`
}

func (e *PrefixEntry) Key() int32 { return e.Order }

type Criteria string

const (
	CriteriaExact   Criteria = "exact"
	CriteriaInexact Criteria = "inexact"
)
