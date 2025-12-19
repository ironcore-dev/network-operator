// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iosxr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*VRF)(nil)

type VRF struct {
	Name       string        `json:"vrf-name"`
	Descr      string        `json:"description"`
	AddrFamily AddressFamily `json:"address-family"`
}

type AddressFamily struct {
	IPv4 UnicastFamily `json:"ipv4,omitzero"`
	IPv6 UnicastFamily `json:"ipv6,omitzero"`
}

type UnicastFamily struct {
	Unicast Unicast `json:"unicast"`
}

type Unicast struct {
	Import RouteTarget `json:"Cisco-IOS-XR-um-router-bgp-cfg:import,omitzero"`
	Export RouteTarget `json:"Cisco-IOS-XR-um-router-bgp-cfg:export,omitzero"`
}

type RouteTarget struct {
	RouteTargetFourByteAS RouteTargetFourByteAsRTS `json:"route-target"`
}

type RouteTargetFourByteAsRTS struct {
	FourByteRT FourByteAsRT `json:"four-byte-as-rts"`
}

type FourByteAsRT struct {
	Target []FourByteRT `json:"four-byte-as-rt"`
}

type FourByteRT struct {
	AsNumber  uint32 `json:"as-number"`
	Index     uint32 `json:"index"`
	Stitching bool   `json:"stitching"`
}

func (v *VRF) XPath() string {
	return "Cisco-IOS-XR-um-vrf-cfg:vrfs/vrf[vrf-name=" + v.Name + "]"
}

func NewRouteTarget(rd string) (FourByteRT, error) {
	parts := strings.SplitN(rd, ":", 2)
	if len(parts) != 2 {
		return FourByteRT{}, fmt.Errorf("invalid rd: %s", rd)
	}

	asn := parts[0]
	index := parts[1]

	asnInt, err := strconv.ParseUint(asn, 10, 32)
	if err != nil {
		return FourByteRT{}, fmt.Errorf("invalid ASN in rd %s: %w", rd, err)
	}

	indexInt, err := strconv.ParseUint(index, 10, 32)
	if err != nil {
		return FourByteRT{}, fmt.Errorf("invalid index in rd %s: %w", rd, err)
	}

	t := FourByteRT{
		AsNumber:  uint32(asnInt),
		Index:     uint32(indexInt),
		Stitching: false,
	}

	return t, nil
}

func AppendAddressFamily(unicast *Unicast, rt *FourByteRT, action v1alpha1.RouteTargetAction) {
	switch action {
	case v1alpha1.RouteTargetActionImport:
		AppendRT(&unicast.Import.RouteTargetFourByteAS.FourByteRT, rt)
	case v1alpha1.RouteTargetActionExport:
		AppendRT(&unicast.Export.RouteTargetFourByteAS.FourByteRT, rt)
	case v1alpha1.RouteTargetActionBoth:
		AppendRT(&unicast.Import.RouteTargetFourByteAS.FourByteRT, rt)
		AppendRT(&unicast.Export.RouteTargetFourByteAS.FourByteRT, rt)
	}
}

func AppendRT(targets *FourByteAsRT, rt *FourByteRT) {
	if len(targets.Target) == 0 {
		targets.Target = []FourByteRT{}
	}
	targets.Target = append(targets.Target, *rt)
}
