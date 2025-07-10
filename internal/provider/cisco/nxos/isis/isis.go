// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// This package provides a representation of an ISIS process on a Cisco NX-OS device.
//
// Both `ToYGOT` and `Reset` return `ReplacingUpdates`. When applied they replace the entire
// `/System/isis-items` subtree on the remote device . Any other existing ISIS processes will
// be removed in this process. If a process with the same name already exists its settings will
// be also completely replaced.
package isis

import (
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.wdf.sap.corp/cc/network-fabric-operator/pkg/gnmiext"
)

var _ gnmiext.DeviceConf = (*ISIS)(nil)

type ISIS struct {
	// name of the ISIS process, e.g., `router isis UNDERLAY`
	Name string
	// Network Entity Title, e.g., `net 49.0001.0001.0000.0001.00`
	NET string
	// type. e.g., `is-type level-1`
	Level ISISType
	//overloadbit options, e.g., `set-overload-bit on-startup 61`
	OverloadBit *OverloadBit
	// supported  families, e.g., `address-family ipv4 unicast` and `address-family ipv6 unicast`
	AddressFamilies []ISISAFType
}

type OverloadBit struct {
	OnStartup uint32
}

//go:generate stringer -type=ISISType
type ISISType int

const (
	Level1 ISISType = iota + 1
	Level2
	Level12
)

type ISISAFType int

const (
	Unknown ISISAFType = iota
	IPv4Unicast
	IPv6Unicast
)

func (i *ISIS) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	isisItems := &nxos.Cisco_NX_OSDevice_System_IsisItems{}

	instList := isisItems.GetOrCreateInstItems().GetOrCreateInstList(i.Name)
	domList := instList.GetOrCreateDomItems().GetOrCreateDomList("default")
	domList.Net = ygot.String(i.NET)
	switch i.Level {
	case Level1:
		domList.IsType = nxos.Cisco_NX_OSDevice_Isis_IsT_l1
	case Level2:
		domList.IsType = nxos.Cisco_NX_OSDevice_Isis_IsT_l2
	case Level12:
		domList.IsType = nxos.Cisco_NX_OSDevice_Isis_IsT_l12
	default:
		return nil, fmt.Errorf("isis: invalid level type %d", i.Level)
	}

	if i.OverloadBit != nil {
		olItems := domList.GetOrCreateOverloadItems()
		olItems.AdminSt = nxos.Cisco_NX_OSDevice_Isis_OverloadAdminSt_bootup
		olItems.StartupTime = ygot.Uint32(i.OverloadBit.OnStartup)
	}

	for af := range i.AddressFamilies {
		switch i.AddressFamilies[af] {
		case IPv4Unicast:
			domList.GetOrCreateAfItems().GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Isis_AfT_v4)
		case IPv6Unicast:
			domList.GetOrCreateAfItems().GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Isis_AfT_v6)
		default:
			return nil, fmt.Errorf("isis: invalid address family type %d", i.AddressFamilies[af])
		}
	}

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/isis-items",
			Value: isisItems,
		},
	}, nil
}

// Reset resets the ISIS configuration to its default state (empty configuration),
// effectively removing all ISIS processes
func (i *ISIS) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	isisItems := &nxos.Cisco_NX_OSDevice_System_IsisItems{}
	isisItems.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/isis-items",
			Value: isisItems,
		},
	}, nil
}
