// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ethernetRe    = regexp.MustCompile(`(?i)^(ethernet|eth)(\d+/\d+)$`)
	loopbackRe    = regexp.MustCompile(`(?i)^(loopback|lo)(\d+)$`)
	portchannelRe = regexp.MustCompile(`(?i)^(port-channel|po)(\d+)$`)
)

func shortNameWithPrefix(name, prefix string, re *regexp.Regexp) (string, error) {
	if name == "" {
		return "", errors.New("interface name must not be empty")
	}
	if re.MatchString(name) {
		matches := re.FindStringSubmatch(name)
		return prefix + matches[2], nil
	}
	return "", fmt.Errorf("unsupported interface format %q, expected %s", name, re.String())
}

// ShortName converts a full interface name to its short form.
// If the name is already in short form, it is returned as is.
func ShortName(name string) (string, error) {
	switch {
	case ethernetRe.MatchString(name):
		return shortNameWithPrefix(name, "eth", ethernetRe)
	case loopbackRe.MatchString(name):
		return shortNameWithPrefix(name, "lo", loopbackRe)
	case portchannelRe.MatchString(name):
		return shortNameWithPrefix(name, "po", portchannelRe)
	default:
		return "", fmt.Errorf("unsupported interface format %q, expected one of: %s, %s, %s", name, ethernetRe.String(), loopbackRe.String(), portchannelRe.String())
	}
}

func ShortNamePortChannel(name string) (string, error) {
	return shortNameWithPrefix(name, "po", portchannelRe)
}

func ShortNamePhysicalInterface(name string) (string, error) {
	return shortNameWithPrefix(name, "eth", ethernetRe)
}

func ShortNameLoopback(name string) (string, error) {
	return shortNameWithPrefix(name, "lo", loopbackRe)
}
