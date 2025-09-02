// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package ospf

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var _ gnmiext.DeviceConf = (*Interface)(nil)

// Interface represents the OSPF configuration pertaining to a device. New interfaces should be created
// using the `NewInterface` function.
type Interface struct {
	interfaceName      string // interface name, e.g., Ethernet1/1
	instanceName       string // name of the OSPF process/instance
	area               string // area ID, an uint32, represented in decimal formar or in decimal-dot notation
	isP2P              bool   // if true, set the network type to P2P, no-op otherwise
	disablePassiveMode bool   // if true, will explicitly disable passive mode on the interface, no-op otherwise
	vrf                string // VRF name, if empty, defaults to "default"
}

func isValidOSPFArea(area string) bool {
	// Try decimal integer
	if n, err := strconv.ParseUint(area, 10, 32); err == nil {
		return n <= math.MaxUint32
	}
	// Try dotted decimal using netip.Addr
	addr, err := netip.ParseAddr(area)
	return err == nil && addr.Is4()
}

type IfOption func(*Interface) error

// NewInterface creates a new OSPF interface configuration for the given interface name, OSPF instance name, and area ID.
// Interface name must be a valid physical or loopback interface name (e.g., Ethernet1/1, lo0). Area ID must be a
// valid uint32 in decimal or dotted decimal notation. Unless specified otherwise, the interface will be configured
// in the default VRF.
func NewInterface(interfaceName, ospfInstanceName, area string, opts ...IfOption) (*Interface, error) {
	shortName, err := iface.ShortName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("ospf: not a valid interface name %q: %w", interfaceName, err)
	}
	if ospfInstanceName == "" {
		return nil, errors.New("ospf: instance name cannot be empty")
	}
	if !isValidOSPFArea(area) {
		return nil, fmt.Errorf("ospf area %s is not a valid uint32 in decimal or dotted decimal notation", area)
	}
	i := &Interface{
		interfaceName: shortName,
		instanceName:  ospfInstanceName,
		area:          area,
		vrf:           "default",
	}
	for _, opt := range opts {
		if err := opt(i); err != nil {
			return nil, err
		}
	}
	return i, nil
}

// WithVRF sets the VRF name for the interface. If not set, defaults to "default".
func WithVRF(vrf string) IfOption {
	return func(i *Interface) error {
		if vrf == "" {
			return errors.New("ospf: vrf name cannot be empty")
		}
		i.vrf = vrf
		return nil
	}
}

// WithP2PNetworkType sets the interface to use OSPF's point-to-point network type
func WithP2PNetworkType() IfOption {
	return func(i *Interface) error {
		i.isP2P = true
		return nil
	}
}

// WithDisablePassiveMode disables passive mode on the interface for OSPF.
func WithDisablePassiveMode() IfOption {
	return func(i *Interface) error {
		i.disablePassiveMode = true
		return nil
	}
}

// ToYGOT returns two updates:
//  1. an editing update to ensure that OSPF is enabled on the device
//  2. a replacing update to re-configure the interface under to use the OSPF instance.
//
// The caller must be aware of the following behavior when applying the updates:
//  1. an OSPF instance will be created if none with such name exists.
//  2. this query checks if the interface exists on the device before attempting applying the configuration. If
//     the interface does not exist or is empty, an error is returned.
//  3. the update will fail if the interface is not configured in layer 3 mode (i.e., equivalent to
//     CLI command `no switchport`). The configuration as a routed interface can be realized
//     using the `iface` package.
func (i *Interface) ToYGOT(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	exists, err := iface.Exists(ctx, client, i.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("ospf: failed to check interface %q existence: %w", i.interfaceName, err)
	}
	if !exists {
		return nil, fmt.Errorf("ospf: interface %q does not exist on the device", i.interfaceName)
	}
	val := &nxos.Cisco_NX_OSDevice_System_OspfItems_InstItems_InstList_DomItems_DomList_IfItems_IfList{
		Area:                 ygot.String(i.area),
		AdvertiseSecondaries: ygot.Bool(true), // NX-OS default behavior (from nx-api sandbox)
		NwT:                  nxos.Cisco_NX_OSDevice_Ospf_NwT_UNSET,
		PassiveCtrl:          nxos.Cisco_NX_OSDevice_Ospf_PassiveControl_UNSET,
	}
	if i.isP2P {
		val.NwT = nxos.Cisco_NX_OSDevice_Ospf_NwT_p2p
	}
	if i.disablePassiveMode {
		val.PassiveCtrl = nxos.Cisco_NX_OSDevice_Ospf_PassiveControl_disabled
	}
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/ospf-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_OspfItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/ospf-items/inst-items/Inst-list[name=" + i.instanceName + "]/dom-items/Dom-list[name=" + i.vrf + "]/if-items/If-list[id=" + i.interfaceName + "]",
			Value: val,
		},
	}, nil
}

// Reset removes the OSPF configuration from the interface by deleting the node associated with the current OSPF instance, the VRF, and the interface name.
func (i *Interface) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/ospf-items/inst-items/Inst-list[name=" + i.instanceName + "]/dom-items/Dom-list[name=" + i.vrf + "]/if-items/If-list[id=" + i.interfaceName + "]",
		},
	}, nil
}
