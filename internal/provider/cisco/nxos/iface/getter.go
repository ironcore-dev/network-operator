// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"fmt"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

// Exists checks if the interface with the given name exists on the device.
func Exists(ctx context.Context, client gnmiext.Client, name string) (bool, error) {
	if name == "" {
		return false, errors.New("interface name must not be empty")
	}
	var err error
	switch {
	case ethernetRe.MatchString(name):
		matches := ethernetRe.FindStringSubmatch(name)
		var inst nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList
		err = client.Get(ctx, "System/intf-items/phys-items/PhysIf-list[id=eth"+matches[2]+"]", &inst)
	case loopbackRe.MatchString(name):
		matches := loopbackRe.FindStringSubmatch(name)
		var inst nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList
		err = client.Get(ctx, "System/intf-items/lb-items/LbRtdIf-list[id=lo"+matches[2]+"]", &inst)
	default:
		return false, fmt.Errorf(`unsupported interface format %q, expected (Ethernet|eth)\d+/\d+ or (Loopback|lo)\d+`, name)
	}
	if err != nil {
		if errors.Is(err, gnmiext.ErrNil) || errors.Is(err, gnmiext.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check interface existence: %w", err)
	}
	return true, nil
}
