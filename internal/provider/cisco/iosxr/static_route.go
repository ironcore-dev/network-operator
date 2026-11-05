// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"
	"strconv"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
)

func (s *Prefix) XPath() string {
	basePath := "Cisco-IOS-XR-um-router-static-cfg:router/static/address-family/"
	if s.VRFName != "" {
		basePath = "Cisco-IOS-XR-um-router-static-cfg:router/static/vrfs/vrf[vrf-name=" + s.VRFName + "]/address-family/"
	}

	if s.IsIpv4 {
		return basePath + "ipv4/unicast/prefixes/prefix[prefix-address=" + s.PrefixAddress + "][prefix-length=" + strconv.Itoa(s.PrefixLength) + "]"
	}
	return basePath + "ipv6/unicast/prefixes/prefix[prefix-address=" + s.PrefixAddress + "][prefix-length=" + strconv.Itoa(s.PrefixLength) + "]"
}

type Prefix struct {
	PrefixAddress    string             `json:"prefix-address"`
	PrefixLength     int                `json:"prefix-length"`
	NextHopAddress   *NexthopAddresses  `json:"nexthop-addresses,omitempty"`
	NextHopInterface *NexthopInterfaces `json:"nexthop-interface-addresses,omitzero"`
	VRFName          string             `json:"-"`
	IsIpv4           bool               `json:"-"`
}

type NexthopAddresses struct {
	NexthopAddress []NexthopAddress `json:"nexthop-address"`
}

type NexthopAddress struct {
	Address  string `json:"address"`
	BFD      *BFD   `json:"bfd,omitempty"`
	Distance uint32 `json:"distance-metric,omitempty"`
}

type NexthopInterfaces struct {
	NexthopInterface []NexthopInterface `json:"nexthop-interface-address,omitempty"`
}

type NexthopInterface struct {
	Address       string `json:"address,omitempty"`
	InterfaceName string `json:"interface-name,omitempty"`
	BFD           *BFD   `json:"bfd,omitempty"`
	Distance      uint32 `json:"distance-metric,omitempty"`
}

// BFD carries the per-nexthop Bidirectional Forwarding Detection configuration.
// A nil BFD (omitted from the payload) means BFD is not configured for the
// nexthop; a non-nil BFD with a FastDetect container enables fast detection.
type BFD struct {
	FastDetect *FastDetect `json:"fast-detect,omitempty"`
}

// FastDetect is the IOS-XR fast-detect presence container. MinimumInterval is
// the hello interval in milliseconds (3..30000) and Multiplier is the detect
// multiplier (1..10). Both are omitted when unset so the device keeps its own
// defaults and a subsequent reconcile does not diff against them.
type FastDetect struct {
	MinimumInterval uint32 `json:"minimum-interval,omitempty"`
	Multiplier      uint32 `json:"multiplier,omitempty"`
}

func NewNexthopAddress(address string, distance *int32, bfd *BFD) NexthopAddress {
	nexthop := NexthopAddress{
		Address: address,
		BFD:     bfd,
	}
	if distance != nil && *distance >= 0 {
		nexthop.Distance = uint32(*distance)
	}
	return nexthop
}

func NewNexthopInterface(name, address string, distance *int32, bfd *BFD) NexthopInterface {
	nexthop := NexthopInterface{
		Address:       address,
		InterfaceName: name,
		BFD:           bfd,
	}
	if distance != nil && *distance >= 0 {
		nexthop.Distance = uint32(*distance)
	}
	return nexthop
}

// IOS-XR fast-detect bounds from the YANG model: minimum-interval is in
// milliseconds and multiplier is the detect multiplier.
const (
	minFastDetectIntervalMs = 3
	maxFastDetectIntervalMs = 30000
	minFastDetectMultiplier = 1
	maxFastDetectMultiplier = 10
)

func NewBFD(bfd *v1alpha1.BFD) (*BFD, error) {
	var violations []apistatus.FieldViolation
	if bfd.RequiredMinimumReceive != nil {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.bfd.requiredMinimumReceive",
			Description: "iosxr provider does not support BFD required minimum receive on static routes",
		})
	}

	fd := &FastDetect{}
	if bfd.DesiredMinimumTxInterval != nil {
		ms := bfd.DesiredMinimumTxInterval.Milliseconds()
		if ms < minFastDetectIntervalMs || ms > maxFastDetectIntervalMs {
			violations = append(violations, apistatus.FieldViolation{
				Field:       "spec.bfd.desiredMinimumTxInterval",
				Description: fmt.Sprintf("must be between %dms and %dms, got %dms", minFastDetectIntervalMs, maxFastDetectIntervalMs, ms),
			})
		} else {
			fd.MinimumInterval = uint32(ms) // #nosec G115 -- bounded to 3..30000 above
		}
	}
	if bfd.DetectionMultiplier != nil {
		m := *bfd.DetectionMultiplier
		if m < minFastDetectMultiplier || m > maxFastDetectMultiplier {
			violations = append(violations, apistatus.FieldViolation{
				Field:       "spec.bfd.detectionMultiplier",
				Description: fmt.Sprintf("must be between %d and %d, got %d", minFastDetectMultiplier, maxFastDetectMultiplier, m),
			})
		} else {
			fd.Multiplier = uint32(m) // #nosec G115 -- bounded to 1..10 above
		}
	}

	if len(violations) > 0 {
		return nil, apistatus.NewInvalidArgumentError(violations...)
	}
	return &BFD{FastDetect: fd}, nil
}
