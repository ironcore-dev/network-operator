// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"errors"
)

type L2Option func(*L2Config) error

type SwitchPortMode uint

const (
	SwitchPortModeAccess SwitchPortMode = iota + 1
	SwitchPortModeTrunk
)

type SpanningTreeMode uint

const (
	SpanningTreeModeUnset SpanningTreeMode = iota
	SpanningTreeModeEdge
	SpanningTreeModeNetwork
	SpanningTreeModeTrunk
)

type L2Config struct {
	spanningTree SpanningTreeMode
	switchPort   SwitchPortMode
	accessVlan   uint16
	nativeVlan   uint16
	allowedVlans []uint16
}

func NewL2Config(opts ...L2Option) (*L2Config, error) {
	if len(opts) == 0 {
		return nil, errors.New("no options provided for L2Config")
	}
	l2cfg := &L2Config{}
	for _, opt := range opts {
		if err := opt(l2cfg); err != nil {
			return nil, err
		}
	}
	return l2cfg, nil
}

func WithSpanningTree(mode SpanningTreeMode) L2Option {
	return func(c *L2Config) error {
		switch mode {
		case SpanningTreeModeUnset, SpanningTreeModeEdge, SpanningTreeModeNetwork, SpanningTreeModeTrunk:
			c.spanningTree = mode
			return nil
		default:
			return errors.New("invalid spanning tree mode")
		}
	}
}

func WithSwithPortMode(mode SwitchPortMode) L2Option {
	return func(c *L2Config) error {
		switch mode {
		case SwitchPortModeAccess, SwitchPortModeTrunk:
			c.switchPort = mode
			return nil
		default:
			return errors.New("invalid switch port mode")
		}
	}
}

func WithAccessVlan(vlan uint16) L2Option {
	return func(c *L2Config) error {
		if c.switchPort != SwitchPortModeAccess {
			return errors.New("access VLAN can only be set for access switch port mode")
		}
		if vlan < 1 || vlan > 4094 {
			return errors.New("access VLAN must be between 1 and 4094")
		}
		c.accessVlan = vlan
		return nil
	}
}

func WithNativeVlan(vlan uint16) L2Option {
	return func(c *L2Config) error {
		if c.switchPort != SwitchPortModeTrunk {
			return errors.New("native VLAN can only be set for trunk switch port mode")
		}
		if vlan < 1 || vlan > 4094 {
			return errors.New("native VLAN must be between 1 and 4094")
		}
		c.nativeVlan = vlan
		return nil
	}
}

func WithAllowedVlans(vlans []uint16) L2Option {
	return func(c *L2Config) error {
		if len(vlans) == 0 {
			return errors.New("number of allowed VLAN should be greater than zero")
		}
		if c.switchPort != SwitchPortModeTrunk {
			return errors.New("allowed VLANs can only be set for trunk switch port mode")
		}
		for _, vlan := range vlans {
			if vlan < 1 || vlan > 4094 {
				return errors.New("allowed VLANs must be between 1 and 4094")
			}
		}
		c.allowedVlans = vlans
		return nil
	}
}
