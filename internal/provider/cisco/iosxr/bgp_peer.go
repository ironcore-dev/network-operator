// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*BGPPeer)(nil)
	_ gnmiext.DataElement = (*BGPPeerOperStatus)(nil)

	_ json.Marshaler   = BGPPeerOperStatus{}
	_ json.Unmarshaler = (*BGPPeerOperStatus)(nil)
)

type AfName string

const (
	AfNameIpv4Unicast   AfName = "ipv4-unicast"
	AfNameIpv4Multicast AfName = "ipv4-multicast"
	AfNameIpv6Unicast   AfName = "ipv6-unicast"
	AfNameIpv6Multicast AfName = "ipv6-multicast"
)

type BGPPeerOperSt string

const (
	BGPPeerOperStIdle        BGPPeerOperSt = "bgp-st-idle"
	BGPPeerOperStConnect     BGPPeerOperSt = "bgp-st-connect"
	BGPPeerOperStActive      BGPPeerOperSt = "bgp-st-active"
	BGPPeerOperStOpenSent    BGPPeerOperSt = "bgp-st-opensent"
	BGPPeerOperStOpenConf    BGPPeerOperSt = "bgp-st-openconfirm"
	BGPPeerOperStEstablished BGPPeerOperSt = "bgp-st-established"
	BGGPeerOperStClosing     BGPPeerOperSt = "bgp-st-closing"
	BGPPeerOperStClosingSyng BGPPeerOperSt = "bgp-st-closing-synrcvd"
)

func (s BGPPeerOperSt) ToSessionState() v1alpha1.BGPPeerSessionState {
	switch s {
	case BGPPeerOperStIdle:
		return v1alpha1.BGPPeerSessionStateIdle
	case BGPPeerOperStConnect:
		return v1alpha1.BGPPeerSessionStateConnect
	case BGPPeerOperStActive:
		return v1alpha1.BGPPeerSessionStateActive
	case BGPPeerOperStOpenSent:
		return v1alpha1.BGPPeerSessionStateOpenSent
	case BGPPeerOperStOpenConf:
		return v1alpha1.BGPPeerSessionStateOpenConfirm
	case BGPPeerOperStEstablished:
		return v1alpha1.BGPPeerSessionStateEstablished
	default:
		return v1alpha1.BGPPeerSessionStateUnknown
	}
}

type BGPPeer struct {
	RouterID  string                   `json:"-"`
	AF        ActivatedAddressFamilies `json:"address-families"`
	Name      string                   `json:"vrf-name"`
	RD        RouteDistinguisher       `json:"rd,omitzero"`
	Neighbors NeighborList             `json:"neighbors,omitzero"`
}

// ActivatedAddressFamilies is required for IOS XR to activate the Address-Family under the BGP process
type ActivatedAddressFamilies struct {
	AF []ActivatedAddressFamily `json:"address-family"`
}

type ActivatedAddressFamily struct {
	AFName       string       `json:"af-name"`
	Redistribute Redistribute `json:"redistribute,omitempty"`
}

type Redistribute struct {
	Static Static `json:"static"`
}

type Static struct{}

type RouteDistinguisher struct {
	TwoByteAS  TwoByteAS   `json:"two-byte-as,omitzero"`
	FourByteAS FourByteAS  `json:"four-byte-as,omitzero"`
	IPAddress  IPAddressAS `json:"ip-address,omitzero"`
}

type IPAddressAS struct {
	Address string `json:"address"`
	Index   int64  `json:"ipv4address-index"`
}

type TwoByteAS struct {
	ASNumber int64 `json:"two-byte-as-number"`
	Index    int64 `json:"asn2-index"`
}

type FourByteAS struct {
	ASNumber int64 `json:"four-byte-as-number"`
	Index    int64 `json:"asn4-index"`
}

type NeighborList struct {
	Neighbors []Neighbor `json:"neighbor"`
}

type Neighbor struct {
	NeighborAddress string                  `json:"address"`
	AF              NeighborAddressFamilies `json:"address-families"`
	LocalAS         LocalAS                 `json:"local-as"`
	RemoteAS        int32                   `json:"remote-as"`
	SessionConfig   SessionConfig           `json:"use"`
}

// Inherit address-family independent config from a session-group
type SessionConfig struct {
	SessionGroup string `json:"session-group,omitzero"`
}

type LocalAS struct {
	AS AS `json:"as"`
}

type AS struct {
	ASNumber  int32     `json:"as-number"`
	NoPrepend PrependAS `json:"no-prepend"`
}

type PrependAS struct {
	ReplaceAS ReplaceAS `json:"replace-as"`
}

type ReplaceAS struct{}

type NeighborAddressFamilies struct {
	AF []NeighborAddressFamily `json:"address-family"`
}

type NeighborAddressFamily struct {
	AfName                    string                    `json:"af-name"`
	MaximumPrefix             MaximumPrefix             `json:"maximum-prefix,omitempty"`
	RoutePolicy               PeeringRPL                `json:"route-policy"`
	SendCommunityEbgp         SendCommunityEbgp         `json:"send-community-ebgp,omitempty"`
	SendCommunityGShutEbgp    SendCommunityGShutEbgp    `json:"send-community-gshut-ebgp,omitempty"`
	SendExtendedCommunityEbgp SendExtendedCommunityEbgp `json:"send-extended-community-ebgp,omitempty"`
	SoftReconfiguration       SoftReconfiguration       `json:"soft-reconfiguration,omitempty"`
}

type SoftReconfiguration struct {
	Inbound Inbound `json:"inbound"`
}

type Inbound struct {
	Always gnmiext.Empty `json:"always"`
}

type SendCommunityEbgp struct{}

type SendCommunityGShutEbgp struct{}

type SendExtendedCommunityEbgp struct{}

type MaximumPrefix struct {
	PrefixLimit int `json:"maximum-prefix-number"`
	Restart     int `json:"restart"`
	Threshold   int `json:"threshold-value"`
}

type PeeringRPL struct {
	In  string `json:"in"`
	Out string `json:"out"`
}

type BGPPeerOperStatus struct {
	Name             string        `json:"-"`
	State            BGPPeerOperSt `json:"connection-state"`
	ConnectionUpTime uint32        `json:"connection-established-time"`
}

func (p *BGPPeerOperStatus) XPath() string {
	return "Cisco-IOS-XR-ipv4-bgp-oper:bgp/instances/instance[instance-name=" + BGPDefaultInstance + "]/instance-active/vrfs/vrf[vrf-name=" + p.Name + "]/sessions/session/connection-state"
}

func (p BGPPeerOperStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.State)
}

func (p *BGPPeerOperStatus) UnmarshalJSON(data []byte) error {
	var t string
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	p.State = BGPPeerOperSt(t)
	return nil
}

func (p *BGPPeer) XPath() string {
	return "Cisco-IOS-XR-um-router-bgp-cfg:router/bgp/instances/instance[instance-name=" + BGPDefaultInstance + "]/as[as-number=" + p.RouterID + "]/vrfs/vrf[vrf-name=" + p.Name + "]"
}

func NewRouteDistinguisher(rd string) (RouteDistinguisher, error) {
	rdObj := RouteDistinguisher{}

	rds := strings.Split(rd, ":")
	if len(rds) != 2 {
		return rdObj, fmt.Errorf("invalid route distinguisher format: %s", rd)
	}

	index, err := strconv.ParseInt(rds[1], 10, 64)
	if err != nil {
		return rdObj, fmt.Errorf("invalid route distinguisher index: %s", rds[1])
	}

	// Type 1: IPv4:Number(0-65535)
	if _, err := netip.ParseAddr(rds[0]); err == nil {
		if index > math.MaxUint16 {
			return rdObj, errors.New("type-1 'Assigned Number' is out of range (0–65535)")
		}
		rdObj.IPAddress = IPAddressAS{
			Address: rds[0],
			Index:   index,
		}
		return rdObj, nil
	}

	asNumber, err := strconv.ParseUint(rds[0], 10, 64)
	if err != nil {
		return rdObj, fmt.Errorf("invalid route distinguisher ASN: %s", rds[0])
	}

	// Type 0: ASN(0-65535):Number(0-4294967295)
	if asNumber <= math.MaxUint16 && index <= math.MaxUint32 {
		rdObj.TwoByteAS = TwoByteAS{
			ASNumber: int64(asNumber),
			Index:    index,
		}
		return rdObj, nil
	}

	// Type 2: ASN(65536-4294967295):Number(0-65535)
	if asNumber <= math.MaxUint32 && index <= math.MaxUint16 {
		rdObj.FourByteAS = FourByteAS{
			ASNumber: int64(asNumber),
			Index:    index,
		}
		return rdObj, nil
	}

	return rdObj, errors.New("not a valid type-0, type-1, or type-2 RD")
}
