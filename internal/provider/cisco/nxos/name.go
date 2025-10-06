// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	mgmtRe        = regexp.MustCompile(`(?i)^mgmt0$`)
	ethernetRe    = regexp.MustCompile(`(?i)^(ethernet|eth)(\d+/\d+)$`)
	loopbackRe    = regexp.MustCompile(`(?i)^(loopback|lo)(\d+)$`)
	portchannelRe = regexp.MustCompile(`(?i)^(port-channel|po)(\d+)$`)
)

// ShortName converts a full interface name to its short form.
// If the name is already in short form, it is returned as is.
func ShortName(name string) (string, error) {
	if name == "" {
		return "", errors.New("interface name must not be empty")
	}
	if matches := ethernetRe.FindStringSubmatch(name); matches != nil {
		return "eth" + matches[2], nil
	}
	if matches := loopbackRe.FindStringSubmatch(name); matches != nil {
		return "lo" + matches[2], nil
	}
	if matches := portchannelRe.FindStringSubmatch(name); matches != nil {
		return "po" + matches[2], nil
	}
	if mgmtRe.MatchString(name) {
		return "mgmt0", nil
	}
	return "", fmt.Errorf("unsupported interface format %q, expected one of: %q, %q, %q, %q", name, mgmtRe.String(), ethernetRe.String(), loopbackRe.String(), portchannelRe.String())
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

func shortNameWithPrefix(name, prefix string, re *regexp.Regexp) (string, error) {
	if name == "" {
		return "", errors.New("interface name must not be empty")
	}
	if matches := re.FindStringSubmatch(name); matches != nil {
		return prefix + matches[2], nil
	}
	return "", fmt.Errorf("invalid interface format %q, expected %q", name, re.String())
}
