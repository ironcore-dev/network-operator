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

func Community(c string) (string, error) {
	s, err := stdcommunity(c)
	if err != nil {
		return "", err
	}
	return "regular:" + s, nil
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

// stdcommunity converts a value to a standard community string.
func stdcommunity(s string) (string, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid bgp community format")
	}
	admin, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid bgp community format: %w", err)
	}
	if admin > math.MaxUint16 {
		return "", fmt.Errorf("standard community 'Administrator' must be in range 0–65535, got %d", admin)
	}
	assigned, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid bgp community format: %w", err)
	}
	if assigned > math.MaxUint16 {
		return "", fmt.Errorf("standard community 'Assigned Number' must be in range 0–65535, got %d", assigned)
	}
	return "as2-nn2:" + s, nil
}

// extcommunity converts a value to an extended community string.
func extcommunity(s string) (string, error) {
	if s == "" {
		return "unknown:0:0", nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid extended community format")
	}
	assigned, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid extended community format: %w", err)
	}
	// Type-1
	if _, err := netip.ParseAddr(parts[0]); err == nil {
		if assigned > math.MaxUint16 {
			return "", fmt.Errorf("extended community 'Assigned Number' must be in range 0–65535, got %d", assigned)
		}
		return "ipv4-nn2:" + s, nil
	}
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid bgp extended community format: %w", err)
	}
	// Type-0
	if asn <= math.MaxUint16 {
		// standard 2-byte ASN
		if assigned <= math.MaxUint16 {
			return "as2-nn2:" + s, nil
		}
		return "as2-nn4:" + s, nil
	}
	// Type-2
	return "as4-nn2:" + s, nil
}
