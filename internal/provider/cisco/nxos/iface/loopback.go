// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"context"
	"errors"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Loopback)(nil)

type Loopback struct {
	name        string
	description *string
	adminSt     bool
	vrf         string
	c           *L3Config
}

type LoopbackOption func(*Loopback) error

// NewLoopbackInterface creates a new loopback interface with the given name and description.
func NewLoopbackInterface(name string, description *string, opts ...LoopbackOption) (*Loopback, error) {
	shortName, err := ShortNameLoopback(name)
	if err != nil {
		return nil, err
	}
	l := &Loopback{
		name:        shortName,
		description: description,
		adminSt:     true,
	}
	for _, opt := range opts {
		if err := opt(l); err != nil {
			return nil, err
		}
	}
	return l, nil
}

func WithLoopbackL3(c *L3Config) LoopbackOption {
	return func(p *Loopback) error {
		if c == nil {
			return errors.New("loopback: l3 configuration cannot be nil")
		}
		if c.addressingMode == AddressingModeUnnumbered {
			return errors.New("loopback: unnumbered addressing mode is not supported for loopback interfaces")
		}
		if c.medium != L3MediumTypeUnset {
			return errors.New("loopback: medium type cannot be set for loopback interfaces")
		}
		p.c = c
		return nil
	}
}

func WithLoopbackVRF(vrf string) LoopbackOption {
	return func(p *Loopback) error {
		if vrf == "" {
			return errors.New("physif: VRF name cannot be empty")
		}
		p.vrf = vrf
		return nil
	}
}

func WithLoopbackAdminState(adminSt bool) LoopbackOption {
	return func(p *Loopback) error {
		p.adminSt = adminSt
		return nil
	}
}

func (l *Loopback) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	ll := &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{
		Descr:   l.description,
		AdminSt: nxos.Cisco_NX_OSDevice_L1_AdminSt_up,
	}
	if !l.adminSt {
		ll.AdminSt = nxos.Cisco_NX_OSDevice_L1_AdminSt_down
	}
	if l.vrf != "" {
		ll.GetOrCreateRtvrfMbrItems().TDn = ygot.String("System/inst-items/Inst-list[name=" + l.vrf + "]")
	}

	updates := []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/intf-items/lb-items/LbRtdIf-list[id=" + l.name + "]",
			Value: ll,
		},
	}

	vrfName := l.vrf
	if vrfName == "" {
		vrfName = "default"
	}

	if l.c != nil {
		l3Updates, err := l.c.ToYGOT(l.name, vrfName)
		if err != nil {
			return nil, fmt.Errorf("loopback: fail to create ygot objects for L3 config %w ", err)
		}
		updates = append(updates, l3Updates...)
	}
	return updates, nil
}

func (l *Loopback) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/intf-items/lb-items/LbRtdIf-list[id=" + l.name + "]",
			Value: &nxos.Cisco_NX_OSDevice_System_IntfItems_LbItems_LbRtdIfList{},
		},
	}, nil
}
