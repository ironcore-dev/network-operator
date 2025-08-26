// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package isis

import (
	"context"
	"errors"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var _ gnmiext.DeviceConf = (*Interface)(nil)

type Interface struct {
	interfaceName string // interface name, e.g., Ethernet1/1
	instanceName  string // name of the ISIS process/instance
	v4Enable      bool   // enable ISIS support for IPv4 address family
	v6Enable      bool   // enable ISIS support for IPv6 address family
	p2p           bool   // set the network type to point-to-point (if false no-op)
	vrf           string // VRF name, if empty, defaults to "default"
}

type IfOption func(*Interface) error

// NewInterface creates a new ISIS interface configuration instance for the given interface name and ISIS instance name.
// Interface name must be a valid physical or loopback interface name (e.g., Ethernet1/1, lo0). Unless specified otherwise,
// the interface will be configured in the default VRF. IPv4 and IPv6 address familites are enabled by default.
func NewInterface(interfaceName, isisInstanceName string, opts ...IfOption) (*Interface, error) {
	shortName, err := iface.ShortName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("isis: not a valid interface name %q: %w", interfaceName, err)
	}
	if isisInstanceName == "" {
		return nil, errors.New("isis: instance name cannot be empty")
	}
	i := &Interface{
		interfaceName: shortName,
		instanceName:  isisInstanceName,
		v4Enable:      true,
		v6Enable:      true,
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
			return errors.New("isis: vrf name cannot be empty")
		}
		i.vrf = vrf
		return nil
	}
}

// WithIPv4 sets the support for the IPv4 address family for ISIS on the interface. Enabled by default.
func WithIPv4(enable bool) IfOption {
	return func(i *Interface) error {
		i.v4Enable = enable
		return nil
	}
}

// WithIPv6 sets the support for the IPv6 address family for ISIS on the interface. Enabled by default.
func WithIPv6(enable bool) IfOption {
	return func(i *Interface) error {
		i.v6Enable = enable
		return nil
	}
}

func WithPointToPoint() IfOption {
	return func(i *Interface) error {
		i.p2p = true
		return nil
	}
}

// ToYGOT returns the YGOT updates to configure the ISIS interface:
//  1. an editing update to ensure that the ISIS feature is enabled on the device
//  2. a replacing update to re-configure the interface to use a given ISIS instance on a given VRF
//
// Note: this does not configure any L3 parameters on the interface. It is assumed that the interface
// has already been configured with an appropriate L3 configuration via the pkg `iface`.
//
// The caller must be aware of the following behavior when applying the updates:
//   - applying the editing update will enable the ISIS feature on the device if not already enabled
//   - the update sets the network type for ISIS as point-to-point
//   - it queries to check if the interface exists on the device before attempting applying the
//     configuration. If the interface does not exist or is empty, an error is returned.
func (i *Interface) ToYGOT(c gnmiext.Client) ([]gnmiext.Update, error) {
	exists, err := iface.Exists(context.Background(), c, i.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("isis: failed to check interface %q existence: %w", i.interfaceName, err)
	}
	if !exists {
		return nil, fmt.Errorf("isis: interface %q does not exist on the device", i.interfaceName)
	}
	value := &nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList{
		Dom:            ygot.String(i.vrf),
		Instance:       ygot.String(i.instanceName),
		V4Enable:       ygot.Bool(i.v4Enable),
		V6Enable:       ygot.Bool(i.v6Enable),
		NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_UNSET,
	}
	if i.p2p {
		value.NetworkTypeP2P = nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_on
	}
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/isis-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_IsisItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/isis-items/if-items/InternalIf-list[id=" + i.interfaceName + "]",
			Value: value,
		},
	}, nil
}

// Reset removes the ISIS configuration from the interface.
func (i *Interface) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/isis-items/if-items/InternalIf-list[id=" + i.interfaceName + "]",
		},
	}, nil
}
