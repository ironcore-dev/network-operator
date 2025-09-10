// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package isis

import (
	"context"
	"errors"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

var _ gnmiext.DeviceConf = (*ISIS)(nil)

type ISIS struct {
	// name of the ISIS process, e.g., `router isis UNDERLAY`
	Name string
	// Network Entity Title, e.g., `net 49.0001.0001.0000.0001.00`
	NET string
	// Level is type. e.g., `is-type level-1`
	Level ISISType
	// overloadbit options
	OverloadBit *OverloadBit
	// supported families
	AddressFamilies []ISISAFType
}

type OverloadBit struct {
	OnStartup uint32
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=ISISType
type ISISType int

const (
	Level1 ISISType = iota + 1
	Level2
	Level12
)

type ISISAFType int

const (
	IPv4Unicast = iota + 1
	IPv6Unicast
)

func (i *ISIS) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if i.Name == "" {
		return nil, errors.New("isis: name must be set")
	}
	if i.NET == "" {
		return nil, errors.New("isis: NET must be set")
	}
	instList := &nxos.Cisco_NX_OSDevice_System_IsisItems_InstItems_InstList{
		Name: ygot.String(i.Name),
	}

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
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/isis-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_IsisItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/isis-items/inst-items/Inst-list[name=" + i.Name + "]",
			Value: instList,
		},
	}, nil
}

// Reset removes the ISIS process with the given name from the device.
func (i *ISIS) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/isis-items/inst-items/Inst-list[name=" + i.Name + "]",
		},
	}, nil
}

var _ ygot.GoStruct = (*AdjStatus)(nil)

type AdjStatus struct {
	ID     *string `json:"id"`
	OperSt string  `json:"operSt"`
}

func (t *AdjStatus) ΛListKeyMap() (map[string]interface{}, error) {
	return map[string]interface{}{
		"id": *t.ID,
	}, nil
}

func (s *AdjStatus) IsYANGGoStruct() {}

func (i ISIS) GetAdjancencyStatus(ctx context.Context, c gnmiext.Client, ifName, vrfName, levelName string) (bool, error) {
	shName, err := iface.ShortNamePhysicalInterface(ifName)
	if err != nil {
		return false, fmt.Errorf("isis: %q is not a valid physical name: %w", shName, err)
	}
	xpath := "System/isis-items/inst-items/Inst-list[name=" + i.Name + "]/dom-items/Dom-list[name=" + vrfName + "]/oper-items/adj-items/level-items/Level-list[cktType=" + levelName + "]/adjif-items/AdjIf-list[id=" + shName + "]/"
	var adjStatus AdjStatus
	err = c.Get(ctx, xpath, &adjStatus, gnmiext.WithType(gpb.GetRequest_STATE), gnmiext.WithStdJSONUnmarshal())
	if err != nil {
		return false, fmt.Errorf("isis: failed to get adjacency status for ISIS instance %q, interface %q, level %q, vrfName %q: %w", i.Name, shName, levelName, vrfName, err)
	}
	return adjStatus.OperSt == "up", nil
}

// GetAdjancencyStatuses returns the number of active and total adjacencies for the given interfaces by repeatedly calling GetAdjancencyStatus. It thus makes multiple gNMI Get calls.
func (i ISIS) GetAdjancencyStatuses(ctx context.Context, c gnmiext.Client, ifNames []string, vrfName, levelName string) (uint16, uint16, error) {
	var active, total uint16
	var errs []error
	for _, ifName := range ifNames {
		up, err := i.GetAdjancencyStatus(ctx, c, ifName, vrfName, levelName)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		total++
		if up {
			active++
		}
	}
	return active, total, errors.Join(errs...)
}
