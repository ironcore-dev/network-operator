// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iosxr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

type StitchingType string

const (
	Enable  StitchingType = "enable"
	Disable StitchingType = "disable"
)

var _ gnmiext.DataElement = (*VRF)(nil)

type VRF struct {
	Name        string        `json:"vrf-name"`
	Description string        `json:"description"`
	AddrFamily  AddressFamily `json:"address-family"`
}

type AddressFamily struct {
	IPv4 UnicastFamily `json:"ipv4,omitzero"`
	IPv6 UnicastFamily `json:"ipv6,omitzero"`
}

type UnicastFamily struct {
	Unicast Unicast `json:"unicast"`
}

type Unicast struct {
	Export RouteTarget `json:"Cisco-IOS-XR-um-router-bgp-cfg:export,omitzero"`
	Import RouteTarget `json:"Cisco-IOS-XR-um-router-bgp-cfg:import,omitzero"`
}

type RouteTarget struct {
	FourByteAS FourByteASRouteTargetList `json:"route-target"`
}

type FourByteASRouteTargetList struct {
	Targets FourByteASRouteTargetWrapper `json:"four-byte-as-route-targets"`
}

type FourByteASRouteTargetWrapper struct {
	Targets []FourByteASRouteTarget `json:"four-byte-as-route-target"`
}

type FourByteASRouteTarget struct {
	ASNumber  uint32        `json:"four-byte-as-number"`
	Index     uint32        `json:"asn4-index"`
	Stitching StitchingType `json:"stitching"`
}

func (v *VRF) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-um-vrf-cfg:vrfs/vrf[vrf-name=%s]", v.Name)
}

func NewFourByteRT(value string) (FourByteASRouteTarget, error) {
	asn, index, found := strings.Cut(value, ":")
	if !found {
		return FourByteASRouteTarget{}, fmt.Errorf("invalid route target format: %q", value)
	}

	asnInt, err := strconv.ParseUint(asn, 10, 32)
	if err != nil {
		return FourByteASRouteTarget{}, fmt.Errorf("invalid ASN in route target %q: %w", value, err)
	}

	indexInt, err := strconv.ParseUint(index, 10, 32)
	if err != nil {
		return FourByteASRouteTarget{}, fmt.Errorf("invalid index in route target %q: %w", value, err)
	}

	return FourByteASRouteTarget{
		ASNumber:  uint32(asnInt),
		Index:     uint32(indexInt),
		Stitching: Disable,
	}, nil
}

func AppendAddressFamily(unicast *Unicast, rt FourByteASRouteTarget, action v1alpha1.RouteTargetAction) {
	switch action {
	case v1alpha1.RouteTargetActionImport:
		AppendRouteTarget(&unicast.Import.FourByteAS.Targets, rt)
	case v1alpha1.RouteTargetActionExport:
		AppendRouteTarget(&unicast.Export.FourByteAS.Targets, rt)
	case v1alpha1.RouteTargetActionBoth:
		AppendRouteTarget(&unicast.Import.FourByteAS.Targets, rt)
		AppendRouteTarget(&unicast.Export.FourByteAS.Targets, rt)
	default:
	}
}

func AppendRouteTarget(wrapper *FourByteASRouteTargetWrapper, rt FourByteASRouteTarget) {
	wrapper.Targets = append(wrapper.Targets, rt)
}
