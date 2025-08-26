// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ethernetRe = regexp.MustCompile(`(?i)^(ethernet|eth)(\d+/\d+)$`)
	loopbackRe = regexp.MustCompile(`(?i)^(loopback|lo)(\d+)$`)
)

// ShortName converts a full interface name to its short form.
// If the name is already in short form, it is returned as is.
func ShortName(name string) (string, error) {
	if name == "" {
		return "", errors.New("interface name must not be empty")
	}
	switch {
	case ethernetRe.MatchString(name):
		matches := ethernetRe.FindStringSubmatch(name)
		return "eth" + matches[2], nil
	case loopbackRe.MatchString(name):
		matches := loopbackRe.FindStringSubmatch(name)
		return "lo" + matches[2], nil
	default:
		return "", fmt.Errorf(`unsupported interface format %q, expected (Ethernet|eth)\d+/\d+ or (Loopback|lo)\d+`, name)
	}
}
