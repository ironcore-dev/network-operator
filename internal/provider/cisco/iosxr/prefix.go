// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ gnmiext.DataElement = (*PrefixList)(nil)

type PrefixList struct {
	Name      string `json:"prefix-list-name"`
	Sequences struct {
		Sequence gnmiext.List[int32, *PrefixEntry] `json:"sequence"`
	} `json:"sequences"`
	// Is6 indicates whether this is an IPv6 prefix list. This field is not serialized to JSON
	// and is only used internally to determine the correct XPath for the prefix list.
	Is6 bool `json:"-"`
}

func (*PrefixList) IsListItem() {}

func (p *PrefixList) XPath() string {
	if p.Is6 {
		return fmt.Sprintf("Cisco-IOS-XR-um-ipv6-prefix-list-cfg:ipv6/prefix-lists/prefix-list[prefix-list-name=%s]", p.Name)
	}
	return fmt.Sprintf("Cisco-IOS-XR-um-ipv4-prefix-list-cfg:ipv4/prefix-lists/prefix-list[prefix-list-name=%s]", p.Name)
}

type PrefixEntry struct {
	SequenceNumber    int32              `json:"sequence-number"`
	Permission        Permission         `json:"permission"`
	Prefix            string             `json:"prefix"`
	Mask              string             `json:"mask"`
	MatchPrefixLength *MatchPrefixLength `json:"match-prefix-length,omitempty"`
}

func (e *PrefixEntry) Key() int32 { return e.SequenceNumber }

type Permission string

const (
	PermissionPermit Permission = "permit"
	PermissionDeny   Permission = "deny"
)

type PrefixType string

const (
	PrefixTypeIPv4 PrefixType = "ipv4"
	PrefixTypeIPv6 PrefixType = "ipv6"
)

type MatchPrefixLength struct {
	EQ int8 `json:"eq,omitempty"`
	GE int8 `json:"ge,omitempty"`
	LE int8 `json:"le,omitempty"`
}
