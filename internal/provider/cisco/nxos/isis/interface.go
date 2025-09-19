// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package isis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var (
	ErrUnsupported error = errors.New("isis: unsupported interface type for isis")
)

type Interface struct {
	name     string // interface name, e.g., Ethernet1/1
	v4Enable bool   // enable ISIS support for IPv4 address family
	v6Enable bool   // enable ISIS support for IPv6 address family
	p2p      bool   // set the network type to point-to-point (if false no-op)
	bfd      bool   // enable BFD for the interface
}

type IfOption func(*Interface) error

// NewInterface creates a new ISIS interface configuration instance for the given interface name and ISIS instance name.
// Interface name must be a valid physical or loopback interface name (e.g., Ethernet1/1, lo0). Unless specified otherwise,
// the interface will be configured in the default VRF. IPv4 and IPv6 address familites are enabled by default.
func NewInterface(name string, opts ...IfOption) (*Interface, error) {
	shortName, err := iface.ShortName(name)
	if err != nil {
		return nil, fmt.Errorf("isis: not a valid interface name %q: %w", name, err)
	}
	if !strings.HasPrefix(shortName, "eth") && !strings.HasPrefix(shortName, "lo") {
		return nil, ErrUnsupported
	}
	i := &Interface{
		name:     shortName,
		v4Enable: true,
		v6Enable: true,
	}
	for _, opt := range opts {
		if err := opt(i); err != nil {
			return nil, err
		}
	}
	return i, nil
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
func WithBFD() IfOption {
	return func(i *Interface) error {
		i.bfd = true
		return nil
	}
}

var ErrInterfaceNotFound = errors.New("interface not found on device")

func (i *Interface) toYGOT(ctx context.Context, client gnmiext.Client) (*nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList_DomItems_DomList_IfItems_IfList, error) {
	exists, err := iface.Exists(ctx, client, i.name)
	if err != nil {
		return nil, fmt.Errorf("isis: failed to check interface %q existence: %w", i.name, err)
	}
	if !exists {
		return nil, ErrInterfaceNotFound
	}
	value := &nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList_DomItems_DomList_IfItems_IfList{
		Id:             ygot.String(i.name),
		V4Enable:       ygot.Bool(i.v4Enable),
		V6Enable:       ygot.Bool(i.v6Enable),
		NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_UNSET,
	}
	if i.p2p {
		value.NetworkTypeP2P = nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_on
	}
	if i.bfd {
		value.V4Bfd = nxos.Cisco_NX_OSDevice_Isis_BfdT_enabled
	}
	return value, nil
}
