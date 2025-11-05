// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*NVE)(nil)
var _ gnmiext.Configurable = (*EVPNStormControl)(nil)
var _ gnmiext.Configurable = (*NVEInfraVLANs)(nil)
var _ gnmiext.Configurable = (*EVPNMultisite)(nil)

// NVE represents the Network Virtualization Edge interface (nve1).
type NVE struct {
	AdminSt          AdminSt       `json:"adminSt"`
	AdvertiseVmac    bool          `json:"advertiseVmac,omitempty"`
	AnycastInterface string        `json:"anycastIntf,omitempty"`
	ID               int           `json:"epId"`
	HoldDownTime     uint16        `json:"holdDownTime,omitempty"`
	HostReach        HostReachType `json:"hostReach,omitempty"`
	McastGroupL2     string        `json:"mcastGroupL2,omitempty"`
	McastGroupL3     string        `json:"mcastGroupL3,omitempty"`
	SourceInterface  string        `json:"sourceInterface,omitempty"`
	SuppressARP      bool          `json:"suppressARP,omitempty"`
}

func (*NVE) IsListItem() {}

func (n *NVE) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=" + strconv.Itoa(n.ID) + "]"
}

type HostReachType string

const (
	HostReachFloodAndLearn HostReachType = "Flood_and_learn"
	HostReachBGP           HostReachType = "bgp"
)

type EVPNStormControlAF string

const (
	EVPNStormControlUnicast   EVPNStormControlAF = "ucast"
	EVPNStormControlBroadcast EVPNStormControlAF = "bcast"
	EVPNStormControlMulticast EVPNStormControlAF = "mcast"
)

// EVPNStormControl represents the storm control settings for EVPN control plane.
type EVPNStormControl struct {
	EvpnStormControlList []*EVPNStormControlEntry `json:"EvpnStormControl-list,omitempty"`
}

type EVPNStormControlEntry struct {
	Name       EVPNStormControlAF `json:"name"`
	FloatLevel string             `json:"floatlevel"`
}

func (*EVPNStormControl) XPath() string {
	return "System/eps-items/stormcontrol-items"
}

type NVEInfraVLANs struct {
	InfraVLANList []*NVEInfraVLAN `json:"InfraVlan-list,omitempty"`
}

func (*NVEInfraVLANs) XPath() string {
	return "System/pltfm-items/nve-items/NVE-list[id=1]/infravlan-items"
}

type NVEInfraVLAN struct {
	ID uint32 `json:"id"`
}

func (*NVEInfraVLAN) IsListItem() {}

type EVPNMultisite struct {
	SiteId uint32  `json:"siteId"`
	State  AdminSt `json:"state"`
}

func (*EVPNMultisite) XPath() string {
	return "System/eps-items/multisite-items"
}
