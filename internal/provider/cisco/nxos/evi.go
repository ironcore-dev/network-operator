// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*BDEVI)(nil)

// BDEVI represents a Bridge Domain Ethernet VPN Instance (MAC-VRF).
type BDEVI struct {
	Encap     string `json:"encap"`
	Rd        string `json:"rd"`
	RttpItems struct {
		RttPList gnmiext.List[RttEntryType, *RttEntry] `json:"RttP-list,omitzero"`
	} `json:"rttp-items,omitzero"`
}

func (*BDEVI) IsListItem() {}

func (b *BDEVI) XPath() string {
	return "System/evpn-items/bdevi-items/BDEvi-list[encap=" + b.Encap + "]"
}

func RouteDistinguisher(rd string) (string, error) {
	s, err := extcommunity(rd)
	if err != nil {
		return "", err
	}
	return "rd:" + s, nil
}

func RouteTarget(rt string) (string, error) {
	s, err := extcommunity(rt)
	if err != nil {
		return "", err
	}
	return "route-target:" + s, nil
}

// extcommunity converts a value to an extended community string.
func extcommunity(s string) (string, error) {
	if s == "" {
		return "unknown:0:0", nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid route distinguisher format")
	}
	asn, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid route distinguisher format: %w", err)
	}
	// Type-0
	if asn > math.MaxUint16 {
		return "as2-nn4:" + s, nil
	}
	// Type-1
	if _, err := netip.ParseAddr(parts[0]); err == nil {
		return "ipv4-nn2:" + s, nil
	}
	// Type-2
	return "as4-nn2:" + s, nil
}
