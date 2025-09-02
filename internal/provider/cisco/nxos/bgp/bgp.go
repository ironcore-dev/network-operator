// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package bgp provides a representation of BGP configuration for Cisco NX-OS devices.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*BGP)(nil)

type BGP struct {
	// The Autonomous System Number of the BGP instance.
	AsNumber string
	// The Router Identifier of the BGP instance, must be an IPv4 address.
	RouterID netip.Addr
	// AddressFamilies is a list of address families configured for the BGP instance.
	AddressFamilies []AddressFamily
}

func NewBGP(routerID string, asn uint32, opts ...BGPOption) (*BGP, error) {
	if routerID == "" {
		return nil, errors.New("bgp: router ID cannot be empty")
	}
	ip, err := netip.ParseAddr(routerID)
	if err != nil {
		return nil, fmt.Errorf("bgp: invalid router ID %q: %w", routerID, err)
	}
	if !ip.Is4() {
		return nil, fmt.Errorf("bgp: router ID must be an IPv4 address, got %q", ip)
	}

	// TODO: support ASNs like '65000.100', ideally with a custom type
	if asn == 0 || asn > 65535 {
		return nil, errors.New("bgp: AS number must be between 1 and 65535")
	}

	bgp := &BGP{
		RouterID: ip,
		AsNumber: strconv.FormatUint(uint64(asn), 10),
	}

	for _, opt := range opts {
		if err := opt(bgp); err != nil {
			return nil, err
		}
	}

	return bgp, nil
}

type BGPOption func(*BGP) error

type AddressFamily interface {
	toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList
}

// WithAddressFamily adds an address family to the BGP configuration.
func WithAddressFamily(af AddressFamily) BGPOption {
	return func(b *BGP) error {
		if af == nil {
			return errors.New("bgp: address family cannot be nil")
		}
		switch af.(type) {
		case *L2EVPN, *IPv4Unicast, *IPv6Unicast:
		default:
			return fmt.Errorf("bgp: unsupported address family type %T", af)
		}
		for _, other := range b.AddressFamilies {
			if other.toYGOT().Type == af.toYGOT().Type {
				return fmt.Errorf("bgp: address family %T already exists", other)
			}
		}
		b.AddressFamilies = append(b.AddressFamilies, af)
		return nil
	}
}

func (b *BGP) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	inst := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems{}
	inst.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	inst.Asn = ygot.String(b.AsNumber)

	dom := inst.GetOrCreateDomItems().GetOrCreateDomList("default")
	dom.RtrId = ygot.String(b.RouterID.String())
	dom.RtrIdAuto = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled

	hasEVPN := false
	for _, af := range b.AddressFamilies {
		_, ok := af.(*L2EVPN)
		hasEVPN = hasEVPN || ok
		if err := dom.GetOrCreateAfItems().AppendDomAfList(af.toYGOT()); err != nil {
			return nil, fmt.Errorf("bgp: failed to append address family: %w", err)
		}
	}

	updates := []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/fm-items/bgp-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/bgp-items/inst-items",
			Value: inst,
		},
	}

	if hasEVPN {
		updates = slices.Insert(updates, 0, gnmiext.Update(gnmiext.ReplacingUpdate{
			XPath: "System/fm-items/evpn-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_EvpnItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		}))
	}

	return updates, nil
}

func (b *BGP) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/bgp-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_BgpItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
			},
		},
		gnmiext.DeletingUpdate{
			XPath: "System/bgp-items",
		},
	}, nil
}

var (
	_ AddressFamily = (*L2EVPN)(nil)
	_ AddressFamily = (*IPv4Unicast)(nil)
	_ AddressFamily = (*IPv6Unicast)(nil)
)

type L2EVPN struct {
	// Forward packets over multipath paths
	MaximumPaths uint8
	// Retain the routes based on Target VPN Extended Communities.
	// Can be "all" to retain all routes, or a specific route-map name.
	RetainRouteTarget string
}

func (evpn *L2EVPN) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn
	if evpn.MaximumPaths > 0 {
		af.MaxExtEcmp = ygot.Uint8(evpn.MaximumPaths)
	}
	if evpn.RetainRouteTarget != "" {
		rtAll, rtMap := nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled, evpn.RetainRouteTarget
		if evpn.RetainRouteTarget == "all" {
			rtAll, rtMap = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled, "DME_UNSET_PROPERTY_MARKER"
		}
		af.RetainRttAll = rtAll
		af.RetainRttRtMap = ygot.String(rtMap)
	}
	return af
}

type IPv4Unicast struct{}

func (*IPv4Unicast) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast
	return af
}

type IPv6Unicast struct{}

func (*IPv6Unicast) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_AfItems_DomAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast
	return af
}

var _ gnmiext.DeviceConf = (*BGPPeer)(nil)

type BGPPeer struct {
	// The BGP Peer's address.
	Addr netip.Addr
	// Neighbor specific description.
	Desc string
	// The Autonomous System Number of the Neighbor
	AsNumber string
	// The local source interface for the BGP session and update messages.
	SrcIf string
	// AddressFamilies is a list of address families configured for the BGP peer.
	AddressFamilies []PeerAddressFamily
}

func NewBGPPeer(addr string, asn uint32, srcIf string, opts ...BGPPeerOption) (*BGPPeer, error) {
	if addr == "" {
		return nil, errors.New("bgp peer: address cannot be empty")
	}
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("bgp peer: invalid address %q: %w", addr, err)
	}

	if asn == 0 || asn > 65535 {
		return nil, errors.New("bgp peer: AS number must be between 1 and 65535")
	}

	if srcIf == "" {
		return nil, errors.New("bgp peer: source interface cannot be empty")
	}

	peer := &BGPPeer{
		Addr:     ip,
		Desc:     "DME_UNSET_PROPERTY_MARKER",
		AsNumber: strconv.FormatUint(uint64(asn), 10),
		SrcIf:    srcIf,
	}
	for _, opt := range opts {
		if err := opt(peer); err != nil {
			return nil, err
		}
	}
	return peer, nil
}

type BGPPeerOption func(*BGPPeer) error

// WithDescription sets the description for the BGP peer.
func WithDescription(desc string) BGPPeerOption {
	return func(p *BGPPeer) error {
		if desc == "" {
			return errors.New("bgp peer: description cannot be empty")
		}
		p.Desc = desc
		return nil
	}
}

type PeerAddressFamily interface {
	toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList
}

// WithPeerAddressFamily adds an address family to the BGP peer configuration.
func WithPeerAddressFamily(af PeerAddressFamily) BGPPeerOption {
	return func(p *BGPPeer) error {
		if af == nil {
			return errors.New("bgp peer: address family cannot be nil")
		}
		switch af.(type) {
		case *PeerL2EVPN, *PeerIPv4Unicast, *PeerIPv6Unicast:
		default:
			return fmt.Errorf("bgp peer: unsupported address family type %T", af)
		}
		for _, other := range p.AddressFamilies {
			if other.toYGOT().Type == af.toYGOT().Type {
				return fmt.Errorf("bgp peer: address family %T already exists", other)
			}
		}
		p.AddressFamilies = append(p.AddressFamilies, af)
		return nil
	}
}

var ErrMissingBGPInstance = errors.New("bgp peer: missing BGP instance")

func (p *BGPPeer) ToYGOT(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	// Ensure that the BGP instance exists and is configured on the "default" domain
	// and return an error if it does not exist.
	// Otherwise, by default of the gnmi specification, all missing nodes in the yang
	// tree would be created, which would mean that we would create a new BGP instance,
	// which is not what we want.
	// Returning an error here allows us to handle the case where the BGP instance is not
	// configured by requeuing the request for the BGP Peer on the k8s controller. This avoids
	// a race condition where the BGP instance is created after the BGP Peer is created.
	var inst nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems
	err := client.Get(ctx, "System/bgp-items/inst-items", &inst)
	if err != nil {
		if errors.Is(err, gnmiext.ErrNil) {
			return nil, ErrMissingBGPInstance
		}
		return nil, fmt.Errorf("bgp peer: failed to get BGP instance: %w", err)
	}
	domList := inst.GetDomItems().GetDomList("default")
	if domList == nil {
		return nil, ErrMissingBGPInstance
	}
	peer := domList.GetOrCreatePeerItems().GetOrCreatePeerList(p.Addr.String())
	peer.Asn = ygot.String(p.AsNumber)
	peer.AsnType = nxos.Cisco_NX_OSDevice_Bgp_PeerAsnType_none
	peer.Name = ygot.String(p.Desc)
	peer.SrcIf = ygot.String(p.SrcIf)
	for _, af := range p.AddressFamilies {
		if err := peer.AfItems.AppendPeerAfList(af.toYGOT()); err != nil {
			return nil, fmt.Errorf("bgp peer: failed to append address family: %w", err)
		}
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr.String() + "]",
			Value: peer,
		},
	}, nil
}

func (p *BGPPeer) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr.String() + "]",
		},
	}, nil
}

var (
	_ PeerAddressFamily = (*PeerL2EVPN)(nil)
	_ PeerAddressFamily = (*PeerIPv4Unicast)(nil)
	_ PeerAddressFamily = (*PeerIPv6Unicast)(nil)
)

type PeerL2EVPN struct {
	// SendStandardCommunity indicates whether to send the standard community attribute.
	SendStandardCommunity bool
	// SendExtendedCommunity indicates whether to send the extended community attribute.
	SendExtendedCommunity bool
	// RouteReflectorClient indicates whether to configure this peer as a route reflector client.
	RouteReflectorClient bool
}

func (p *PeerL2EVPN) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn
	af.SendComStd = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled
	if p.SendStandardCommunity {
		af.SendComStd = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled
	}
	af.SendComExt = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_disabled
	if p.SendExtendedCommunity {
		af.SendComExt = nxos.Cisco_NX_OSDevice_Bgp_AdminSt_enabled
	}
	af.Ctrl = ygot.String("DME_UNSET_PROPERTY_MARKER")
	if p.RouteReflectorClient {
		af.Ctrl = ygot.String("rr-client")
	}
	return af
}

type PeerIPv4Unicast struct{}

func (p *PeerIPv4Unicast) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast
	return af
}

type PeerIPv6Unicast struct{}

func (p *PeerIPv6Unicast) toYGOT() *nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList {
	af := &nxos.Cisco_NX_OSDevice_System_BgpItems_InstItems_DomItems_DomList_PeerItems_PeerList_AfItems_PeerAfList{}
	af.Type = nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast
	return af
}
