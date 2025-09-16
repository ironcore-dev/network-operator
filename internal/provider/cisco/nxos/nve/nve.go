// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nve

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var _ gnmiext.DeviceConf = (*NVE)(nil)

type HostReachType uint

const (
	HostReachFloodAndLearn HostReachType = iota + 1
	HostReachBGP
)

var (
	ErrInvalidInterfaceName    = errors.New("nve: invalid interface name")
	ErrInvalidInterfaceNumber  = errors.New("nve: interface number must be between 0 and 1023")
	ErrInvalidHoldDownTime     = errors.New("nve: hold down time must be between 1 and 1500 seconds")
	ErrInvalidHostReachProto   = errors.New("nve: invalid host reachability protocol")
	ErrMissingSourceInterface  = errors.New("nve: source interface must be set before setting anycast interface")
	ErrIdenticalInterfaceNames = errors.New("nve: source and anycast interface names must be different")
	ErrInvalidFormatIPAddress  = errors.New("nve: failed to parse IP address")
)

// NVE represents the Network Virtualization Edge interface (nve1). This object must be
// initialized with the NewNVE function.
type NVE struct {
	adminSt              bool
	hostReach            HostReachType
	advertiseVirtualRmac *bool
	// the name of the loopback to use as source
	sourceInterface string
	// the name of the loopback to use for anycast
	anycastInterface string
	suppressARP      *bool
	// multicast group for L2 VTEP discovery
	mcastL2 *netip.Addr
	// multicast group for L3 VTEP discovery
	mcastL3      *netip.Addr
	holdDownTime uint16 // in seconds
}

type NVEOption func(*NVE) error

func NewNVE(opts ...NVEOption) (*NVE, error) {
	n := &NVE{
		adminSt: true,
	}
	for _, opt := range opts {
		if err := opt(n); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// WithAdminState sets the administrative state of the NVE interface. If not set, the default is `up`.
func WithAdminState(adminSt bool) NVEOption {
	return func(n *NVE) error {
		n.adminSt = adminSt
		return nil
	}
}

// WithHostReachabilityProtocol sets the host reachability protocol for the NVE.
func WithHostReachabilityProtocol(proto HostReachType) NVEOption {
	return func(n *NVE) error {
		switch proto {
		case HostReachBGP, HostReachFloodAndLearn:
			n.hostReach = proto
		default:
			return ErrInvalidHostReachProto
		}
		return nil
	}
}

// WithAdvertiseVirtualRmac enables or disables the advertisement of the virtual RMAC address for the NVE interface.
func WithAdvertiseVirtualRmac(enable bool) NVEOption {
	return func(n *NVE) error {
		n.advertiseVirtualRmac = ygot.Bool(enable)
		return nil
	}
}

// WithSourceInterface sets the source interface for the NVE. It must be a loopback interface as per the naming convention
// defined in the `iface` package.
func WithSourceInterface(loopback string) NVEOption {
	return func(n *NVE) error {
		loName, err := iface.ShortNameLoopback(loopback)
		if err != nil {
			return ErrInvalidInterfaceName
		}
		loNr, err := strconv.Atoi(loName[2:])
		if err != nil || loNr > 1023 {
			return ErrInvalidInterfaceNumber
		}
		n.sourceInterface = loName
		return nil
	}
}

// WithAnycastInterface sets the anycast interface for the NVE. It must be a loopback interface as per the naming convention
// defined in the `iface` package. The anycast interface must be different from the source interface. The source interface
// must be set before setting the anycast interface.
func WithAnycastInterface(loopback string) NVEOption {
	return func(n *NVE) error {
		if n.sourceInterface == "" {
			return ErrMissingSourceInterface
		}
		loName, err := iface.ShortNameLoopback(loopback)
		if err != nil {
			return ErrInvalidInterfaceName
		}
		loNr, err := strconv.Atoi(loName[2:])
		if err != nil || loNr > 1023 {
			return ErrInvalidInterfaceNumber
		}
		if loName == n.sourceInterface {
			return ErrIdenticalInterfaceNames
		}
		n.anycastInterface = loName
		return nil
	}
}

// WithSuppressARP sets the NVE to suppress ARP requests for VTEP IP addresses. If not set, the NVE will
// use the default behavior. When set, this is the equivalent to the configuration statement `global suppress-arp`.
// This config will not be shown with `show running-config interface nve1` but only over gNMI.
func WithSuppressARP(enable bool) NVEOption {
	return func(n *NVE) error {
		n.suppressARP = ygot.Bool(enable)
		return nil
	}
}

func validateMulticastAddress(addr string) (*netip.Addr, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, ErrInvalidFormatIPAddress
	}
	if !ip.Is4() || !ip.IsMulticast() {
		return nil, fmt.Errorf("nve: invalid multicast IPv4 address: %s", addr)
	}
	return &ip, nil
}

// WithMulticastGroupL2 configures the global multicast group for the Layer 2 VNI.
// Addr must be a valid IPv4 multicast address.
func WithMulticastGroupL2(addr string) NVEOption {
	return func(n *NVE) error {
		ip, err := validateMulticastAddress(addr)
		if err != nil {
			return err
		}
		n.mcastL2 = ip
		return nil
	}
}

// WithMulticastGroupL3 configures the global multicast group for the Layer 3 VNI.
// Addr must be a valid IPv4 multicast address.
func WithMulticastGroupL3(addr string) NVEOption {
	return func(n *NVE) error {
		ip, err := validateMulticastAddress(addr)
		if err != nil {
			return err
		}
		n.mcastL3 = ip
		return nil
	}
}

// WithHoldDownTime sets the hold down time for the NVE interface in seconds (1-1500).
func WithHoldDownTime(seconds uint16) NVEOption {
	return func(n *NVE) error {
		if seconds < 1 || seconds > 1500 {
			return ErrInvalidHoldDownTime
		}
		n.holdDownTime = seconds
		return nil
	}
}

// ToYGOT converts the NVE configuration to these gNMI updates:
//   - enable the NV feature on the device
//   - configure the NVE interface with the provided settings
//   - enable the NG-MVPN feature (only if a multicast group for L3 is set)
func (n *NVE) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	updates := []gnmiext.Update{}
	val := nxos.Cisco_NX_OSDevice_System_EpsItems_EpIdItems_EpList{}

	val.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	if !n.adminSt {
		val.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled
	}

	switch n.hostReach {
	case HostReachBGP:
		val.HostReach = nxos.Cisco_NX_OSDevice_Nvo_HostReachT_bgp
	case HostReachFloodAndLearn:
		val.HostReach = nxos.Cisco_NX_OSDevice_Nvo_HostReachT_Flood_and_learn
	default:
		// No-op
	}

	val.AdvertiseVmac = n.advertiseVirtualRmac

	if n.sourceInterface != "" {
		val.SourceInterface = ygot.String(n.sourceInterface)
		if n.anycastInterface != "" {
			val.AnycastIntf = ygot.String(n.anycastInterface)
		}
	}

	val.SuppressARP = n.suppressARP
	adminSt := nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled
	if !n.adminSt {
		adminSt = nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled
	}
	if n.mcastL2 != nil {
		val.McastGroupL2 = ygot.String(n.mcastL2.String())
	}
	if n.mcastL3 != nil {

		updates = append(updates, gnmiext.EditingUpdate{
			XPath: "/System/fm-items/ngmvpn-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
				AdminSt: adminSt,
			},
		})
		val.McastGroupL3 = ygot.String(n.mcastL3.String())
	}

	if n.holdDownTime != 0 {
		val.HoldDownTime = ygot.Uint16(n.holdDownTime)
	}
	return append(updates, []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "/System/fm-items/nvo-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
				AdminSt: adminSt,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
			Value: &val,
		},
	}...), nil
}

// Reset removes the nve1 interface and disables the NV and NGMVPN feature on the device.
func (n *NVE) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "/System/eps-items/epId-items/Ep-list[epId=1]",
		},
		gnmiext.EditingUpdate{
			XPath: "/System/fm-items/ngmvpn-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NgmvpnItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
			},
		},
		gnmiext.EditingUpdate{
			XPath: "/System/fm-items/nvo-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NvoItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
			},
		},
	}, nil
}
