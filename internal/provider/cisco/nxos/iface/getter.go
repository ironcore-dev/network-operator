// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var patterns = map[string]*regexp.Regexp{
	"System/intf-items/phys-items/PhysIf-list[id=eth%s]": ethernetRe,
	"System/intf-items/lb-items/LbRtdIf-list[id=lo%s]":   loopbackRe,
	"System/intf-items/aggr-items/AggrIf-list[id=po%s]":  portchannelRe,
}

// Exists checks if the interface with the given name exists on the device
func Exists(ctx context.Context, client gnmiext.Client, name string) (bool, error) {
	if name == "" {
		return false, errors.New("interface name must not be empty")
	}
	for path, re := range patterns {
		if re.MatchString(name) {
			matches := re.FindStringSubmatch(name)
			xpath := fmt.Sprintf(path, matches[2])
			return client.Exists(ctx, xpath)
		}
	}
	return false, fmt.Errorf(`unsupported interface format %q, expected (Ethernet|eth)\d+/\d+ or (Loopback|lo)\d+`, name)
}
