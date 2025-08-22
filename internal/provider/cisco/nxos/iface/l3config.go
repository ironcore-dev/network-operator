// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

type L3Option func(*L3Config) error

type L3MediumType int

const (
	L3MediumTypeUnset L3MediumType = iota
	L3MediumTypeBroadcast
	L3MediumTypeP2P
)

type L3IPAddressingModeType int

const (
	AddressingModeNumbered L3IPAddressingModeType = iota + 1
	AddressingModeUnnumbered
)

type ISISConfig struct {
	Name     string // e.g., "UNDERLAY"
	V4Enable bool
	V6Enable bool
}

type L3Config struct {
	medium             L3MediumType
	addressingMode     L3IPAddressingModeType
	unnumberedLoopback string // used with unnumbered addressing: name of the loopback interface name we borrow the IP from
	prefixesIPv4       []netip.Prefix
	prefixesIPv6       []netip.Prefix
	isisCfg            *ISISConfig
	pimSparseMode      bool
}

func NewL3Config(opts ...L3Option) (*L3Config, error) {
	cfg := &L3Config{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func WithSparseModePIM() L3Option {
	return func(c *L3Config) error {
		c.pimSparseMode = true
		return nil
	}
}

// WithISIS configures the ISIS routing protocol for the interface.
// This is a no-op if a process with such name does not exist.
func WithISIS(name string, v4Enable, v6Enable bool) L3Option {
	return func(c *L3Config) error {
		c.isisCfg = &ISISConfig{
			Name:     name,
			V4Enable: v4Enable,
			V6Enable: v6Enable,
		}
		return nil
	}
}

// WithUnnumberedAddressing sets the interface to use unnumbered addressing, borrowing the IP from the specified loopback interface.
// If the interface where this config is applied is not configured to be in the medium P2P, an error is returned.
func WithUnnumberedAddressing(interfaceName string) L3Option {
	return func(c *L3Config) error {
		loName, err := getLoopbackShortName(interfaceName)
		if err != nil {
			return fmt.Errorf("not a valid loopback interface name %s", interfaceName)
		}
		if c.medium != L3MediumTypeP2P {
			return errors.New("interface must use P2P medium type for unnumbered addressing")
		}
		c.addressingMode = AddressingModeUnnumbered
		c.unnumberedLoopback = loName
		c.prefixesIPv4 = nil
		c.prefixesIPv6 = nil
		return nil
	}
}

// WithNumberedAddressingIPv4 sets the interface to use numbered addressing with the provided IPv4 addresses.
// Returns an error if the addresses are empty, invalid or overlap.
func WithNumberedAddressingIPv4(v4prefixes []string) L3Option {
	return func(c *L3Config) error {
		if len(v4prefixes) == 0 {
			return errors.New("at least one IPv4 prefix must be provided for numbered addressing")
		}
		var parsed []netip.Prefix
		for _, prefixStr := range v4prefixes {
			prefix, err := netip.ParsePrefix(prefixStr)
			if err != nil || !prefix.Addr().Is4() {
				return fmt.Errorf("invalid IPv4 prefix %s: %w", prefixStr, err)
			}
			parsed = append(parsed, prefix)
		}
		for i := range parsed {
			for j := i + 1; j < len(parsed); j++ {
				if parsed[i].Overlaps(parsed[j]) {
					return fmt.Errorf("overlapping IPv4 prefixes: %s and %s", parsed[i], parsed[j])
				}
			}
		}
		c.addressingMode = AddressingModeNumbered
		c.prefixesIPv4 = parsed
		c.unnumberedLoopback = ""
		return nil
	}
}

// WithNumberedAddressingIPv6 sets the interface to use numbered addressing with the provided IPv6 addresses.
// Returns an error if any of the prefixes are empty, invalid or overlap.
func WithNumberedAddressingIPv6(v6prefixes []string) L3Option {
	return func(c *L3Config) error {
		if len(v6prefixes) == 0 {
			return errors.New("at least one IPv4 prefix must be provided for numbered addressing")
		}
		var parsed []netip.Prefix
		for _, prefixStr := range v6prefixes {
			prefix, err := netip.ParsePrefix(prefixStr)
			if err != nil || !prefix.Addr().Is6() {
				return fmt.Errorf("invalid IPv6 prefix %s: %w", prefixStr, err)
			}
			parsed = append(parsed, prefix)
		}
		for i := range parsed {
			for j := i + 1; j < len(parsed); j++ {
				if parsed[i].Overlaps(parsed[j]) {
					return fmt.Errorf("overlapping IPv6 prefixes: %s and %s", parsed[i], parsed[j])
				}
			}
		}
		c.addressingMode = AddressingModeNumbered
		c.prefixesIPv6 = parsed
		c.unnumberedLoopback = ""
		return nil
	}
}

// WithMedium sets the L3 medium type for the interface.
func WithMedium(medium L3MediumType) L3Option {
	return func(i *L3Config) error {
		switch medium {
		case L3MediumTypeUnset, L3MediumTypeBroadcast, L3MediumTypeP2P:
			i.medium = medium
			return nil
		default:
			return fmt.Errorf("invalid L3 medium type: %v", medium)
		}
	}
}

func (c *L3Config) ToYGOT(interfaceName, vrfName string) ([]gnmiext.Update, error) {
	updates := []gnmiext.Update{}
	if c.pimSparseMode {
		updates = append(updates, c.createPIM(interfaceName, vrfName))
	}
	switch c.addressingMode {
	case AddressingModeUnnumbered:
		updates = append(updates, c.createAddressingUnnumbered(interfaceName, vrfName))
	case AddressingModeNumbered:
		if len(c.prefixesIPv4) > 0 {
			updates = append(updates, c.createAddressingIP4(interfaceName, vrfName))
		}
		if len(c.prefixesIPv6) > 0 {
			updates = append(updates, c.createAddressingIP6(interfaceName, vrfName))
		}
	}
	if c.isisCfg != nil {
		isisUpdate, err := c.createISIS(interfaceName, vrfName)
		if err != nil {
			return nil, fmt.Errorf("L3: fail to create ygot objects for ISIS config %w ", err)
		}
		if isisUpdate != nil {
			updates = append(updates, isisUpdate)
		}
	}
	return updates, nil
}

// createPIM configures PIM in sparse mode for the interface
func (c *L3Config) createPIM(interfaceName, vrfName string) gnmiext.Update {
	return gnmiext.ReplacingUpdate{
		XPath: "System/pim-items/inst-items/dom-items/Dom-list[name=" + vrfName + "]/if-items/If-list[id=" + interfaceName + "]",
		Value: &nxos.Cisco_NX_OSDevice_System_PimItems_InstItems_DomItems_DomList_IfItems_IfList{
			AdminSt:       nxos.Cisco_NX_OSDevice_Nw_IfAdminSt_enabled,
			PimSparseMode: ygot.Bool(true),
		},
	}
}

func (c *L3Config) createAddressingUnnumbered(interfaceName, vrfName string) gnmiext.Update {
	iface := &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{}
	iface.Unnumbered = ygot.String(c.unnumberedLoopback)
	return gnmiext.ReplacingUpdate{
		XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=" + vrfName + "]/if-items/If-list[id=" + interfaceName + "]",
		Value: iface,
	}
}

// createAddressingIP4 returns updates to configure l3 addressing on the interface (IPv4).
func (c *L3Config) createAddressingIP4(interfaceName, vrfName string) gnmiext.Update {
	iface := &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList{}
	for _, addr := range c.prefixesIPv4 {
		iface.GetOrCreateAddrItems().GetOrCreateAddrList(addr.String())
	}
	return gnmiext.ReplacingUpdate{
		XPath: "System/ipv4-items/inst-items/dom-items/Dom-list[name=" + vrfName + "]/if-items/If-list[id=" + interfaceName + "]",
		Value: iface,
	}
}

// createAddressingIP6 returns updates to configure l3 addressing on the interface (IPv6).
func (c *L3Config) createAddressingIP6(interfaceName, vrfName string) gnmiext.Update {
	iface := &nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList{}
	for _, addr := range c.prefixesIPv6 {
		iface.GetOrCreateAddrItems().GetOrCreateAddrList(addr.String())
	}
	return gnmiext.ReplacingUpdate{
		XPath: "System/ipv6-items/inst-items/dom-items/Dom-list[name=" + vrfName + "]/if-items/If-list[id=" + interfaceName + "]",
		Value: iface,
	}
}

func (c *L3Config) createISIS(interfaceName, vrfName string) (gnmiext.Update, error) {
	if c.isisCfg == nil {
		return nil, nil
	}
	if c.isisCfg.Name == "" {
		return nil, errors.New("isis config name is not set")
	}
	return gnmiext.ReplacingUpdate{
		XPath: "System/isis-items/if-items/InternalIf-list[id=" + interfaceName + "]",
		Value: &nxos.Cisco_NX_OSDevice_System_IsisItems_IfItems_InternalIfList{
			Dom:            ygot.String(vrfName),
			Instance:       ygot.String(c.isisCfg.Name),
			NetworkTypeP2P: nxos.Cisco_NX_OSDevice_Isis_NetworkTypeP2PSt_on,
			V4Enable:       ygot.Bool(c.isisCfg.V4Enable),
			V6Enable:       ygot.Bool(c.isisCfg.V6Enable),
		},
	}, nil
}
