// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package pim provides a representation of Protocol Independent Multicast (PIM) configuration for Cisco NX-OS devices.
package pim

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var _ gnmiext.DeviceConf = (*RendezvousPoint)(nil)

type RendezvousPoint struct {
	// Addr is the IP address of the rendezvous point.
	Addr netip.Addr
	// Group is the static multicast address range for which this rendezvous point is configured.
	Group netip.Prefix
	// Vrf is the VRF in which to configure the rendezvous point, e.g., "default".
	Vrf string
}

func NewRendezvousPoint(addr string, opts ...RendezvousPointOption) (*RendezvousPoint, error) {
	if addr == "" {
		return nil, errors.New("pim: rendezvous point address cannot be empty")
	}
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("pim: failed to parse rendezvous point address %q: %w", addr, err)
	}
	if !a.IsValid() || !a.Is4() {
		return nil, fmt.Errorf("pim: rendezvous point address %q is not a valid IPv4 address", addr)
	}
	rp := &RendezvousPoint{
		Addr: a,
		Vrf:  "default",
	}
	for _, opt := range opts {
		if err := opt(rp); err != nil {
			return nil, err
		}
	}
	return rp, nil
}

type RendezvousPointOption func(*RendezvousPoint) error

// WithGroupList adds a static group range to the rendezvous point configuration.
func WithGroupList(group string) RendezvousPointOption {
	return func(p *RendezvousPoint) error {
		if group == "" {
			return errors.New("pim: group list cannot be empty")
		}
		prefix, err := netip.ParsePrefix(group)
		if err != nil {
			return fmt.Errorf("pim: failed to parse group list %q: %w", group, err)
		}
		if !prefix.IsValid() || !prefix.Addr().Is4() {
			return fmt.Errorf("pim: group list %q is not a valid IPv4 address prefix", group)
		}
		p.Group = prefix
		return nil
	}
}

// WithRendezvousPointVRF configures the VRF in which to configure the rendezvous point.
func WithRendezvousPointVRF(vrf string) RendezvousPointOption {
	return func(p *RendezvousPoint) error {
		if vrf == "" {
			return errors.New("pim: vrf cannot be empty")
		}
		p.Vrf = vrf
		return nil
	}
}

// CIDR returns the CIDR notation ("<ip>/<bits>") of the rendezvous points IPv4 address.
// It returns an error if the address is not a valid IPv4 address.
func (p *RendezvousPoint) CIDR() (string, error) {
	if !p.Addr.IsValid() || !p.Addr.Is4() {
		return "", fmt.Errorf("pim: rendezvous point address %q is not a valid IPv4 address", p.Addr)
	}
	prefix, err := p.Addr.Prefix(32)
	if err != nil {
		return "", fmt.Errorf("pim: failed to create prefix for rendezvous point address %q: %w", p.Addr, err)
	}
	return prefix.String(), nil
}

func (p *RendezvousPoint) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	cidr, err := p.CIDR()
	if err != nil {
		return nil, err
	}
	rp := &nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_StaticrpItems_RpItems_StaticRPList{}
	rp.Addr = ygot.String(cidr)
	if p.Group.IsValid() {
		list := rp.GetOrCreateRpgrplistItems().GetOrCreateRPGrpListList(p.Group.String())
		list.Bidir = ygot.Bool(false)
		list.Override = ygot.Bool(false)
	}
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/pim-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_PimItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + p.Vrf + "]/staticrp-items/rp-items/StaticRP-list[addr=" + cidr + "]",
			Value: rp,
		},
	}, nil
}

func (p *RendezvousPoint) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	cidr, err := p.CIDR()
	if err != nil {
		return nil, err
	}
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + p.Vrf + "]/staticrp-items/rp-items/StaticRP-list[addr=" + cidr + "]",
		},
	}, nil
}

var _ gnmiext.DeviceConf = (*AnycastPeer)(nil)

// AnycastPeer represents a PIM anycast rendezvous point peer configuration.
// It is used to configure anycast rendezvous point peers for redundancy.
type AnycastPeer struct {
	Addr netip.Addr
	// Vrf is the VRF in which to configure the anycast peer, e.g., "default".
	Vrf string
}

func NewAnycastPeer(addr string, opts ...AnycastPeerOption) (*AnycastPeer, error) {
	if addr == "" {
		return nil, errors.New("pim: anycast rendezvous point address cannot be empty")
	}
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("pim: failed to parse anycast rendezvous point address %q: %w", addr, err)
	}
	if !a.IsValid() || !a.Is4() {
		return nil, fmt.Errorf("pim: anycast rendezvous point address %q is not a valid IPv4 address", addr)
	}
	ap := &AnycastPeer{
		Addr: a,
		Vrf:  "default",
	}
	for _, opt := range opts {
		if err := opt(ap); err != nil {
			return nil, err
		}
	}
	return ap, nil
}

type AnycastPeerOption func(*AnycastPeer) error

// WithAnycastPeerVRF configures the VRF in which to configure the anycast peer.
func WithAnycastPeerVRF(vrf string) AnycastPeerOption {
	return func(a *AnycastPeer) error {
		if vrf == "" {
			return errors.New("pim: vrf cannot be empty")
		}
		a.Vrf = vrf
		return nil
	}
}

// CIDR returns the CIDR notation ("<ip>/<bits>") of the anycast rendezvous point IPv4 address.
// It returns an error if the address is not a valid IPv4 address.
func (a *AnycastPeer) CIDR() (string, error) {
	if !a.Addr.IsValid() || !a.Addr.Is4() {
		return "", fmt.Errorf("pim: anycast rendezvous point address %q is not a valid IPv4 address", a.Addr)
	}
	prefix, err := a.Addr.Prefix(32)
	if err != nil {
		return "", fmt.Errorf("pim: failed to create prefix for anycast rendezvous point address %q: %w", a.Addr, err)
	}
	return prefix.String(), nil
}

func (a *AnycastPeer) ToYGOT(client gnmiext.Client) ([]gnmiext.Update, error) {
	cidr, err := a.CIDR()
	if err != nil {
		return nil, err
	}
	ap := &nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_AcastrpfuncItems_PeerItems_AcastRPPeerList{}
	ap.Addr = ygot.String(cidr)
	ap.RpSetAddr = ygot.String(cidr)
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/pim-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_PimItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + a.Vrf + "]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=" + cidr + "][rpSetAddr=" + cidr + "]",
			Value: ap,
		},
	}, nil
}

func (a *AnycastPeer) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	cidr, err := a.CIDR()
	if err != nil {
		return nil, err
	}
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + a.Vrf + "]/acastrpfunc-items/peer-items/AcastRPPeer-list[addr=" + cidr + "][rpSetAddr=" + cidr + "]",
		},
	}, nil
}

var _ gnmiext.DeviceConf = (*Interface)(nil)

// Interface represents a PIM interface configuration. It is used to configure PIM on a specific interface.
type Interface struct {
	// Name is the short name of the interface, e.g., "eth1/2" or "lo0".
	Name string
	// SparseMode indicates whether the interface should be configured for sparse mode.
	SparseMode bool
	// Vrf is the VRF in which to configure PIM, e.g., "default".
	Vrf string
}

func NewInterface(name string, opts ...InterfaceOption) (*Interface, error) {
	shortName, err := iface.ShortName(name)
	if err != nil {
		return nil, fmt.Errorf("pim: invalid interface name %q: %w", name, err)
	}
	i := &Interface{
		Name: shortName,
		Vrf:  "default",
	}
	for _, opt := range opts {
		if err := opt(i); err != nil {
			return nil, err
		}
	}
	return i, nil
}

// InterfaceOption is a function that can be used to configure an Interface.
type InterfaceOption func(*Interface) error

// WithSparseMode configures the interface for PIM sparse mode.
func WithSparseMode(enable bool) InterfaceOption {
	return func(i *Interface) error {
		i.SparseMode = enable
		return nil
	}
}

// WithInterfaceVRF configures the VRF in which to configure PIM on the interface.
func WithInterfaceVRF(vrf string) InterfaceOption {
	return func(i *Interface) error {
		if vrf == "" {
			return errors.New("pim: vrf cannot be empty")
		}
		i.Vrf = vrf
		return nil
	}
}

// ErrMissingInterface is returned when the specified interface does not exist on the device.
// This can happen when trying to configure PIM on an interface that is not present in the
// device's configuration (yet).
var ErrMissingInterface = errors.New("pim: missing interface")

func (i *Interface) ToYGOT(client gnmiext.Client) ([]gnmiext.Update, error) {
	ctx := context.Background()
	exists, err := iface.Exists(ctx, client, i.Name)
	if err != nil {
		return nil, fmt.Errorf("pim: failed to get interface %q: %w", i.Name, err)
	}
	if !exists {
		return nil, ErrMissingInterface
	}
	intf := &nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList{}
	intf.PopulateDefaults()
	intf.Ctrl = ygot.String("border")
	intf.PimSparseMode = ygot.Bool(i.SparseMode)
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/pim-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_PimItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.EditingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + i.Vrf + "]/if-items/If-list[id=" + i.Name + "]",
			Value: intf,
		},
	}, nil
}

func (i *Interface) Reset(client gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + i.Vrf + "]/if-items/If-list[id=" + i.Name + "]",
		},
	}, nil
}
