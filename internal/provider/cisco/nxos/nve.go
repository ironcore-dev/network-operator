// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*NVE)(nil)

// NVE represents the Network Virtualization Edge interface (nve1).
type NVE struct {
	AdminSt          AdminSt       `json:"adminSt"`
	AdvertiseVmac    bool          `json:"advertiseVmac"`
	AnycastInterface string        `json:"anycastIntf,omitempty"`
	ID               int           `json:"epId"`
	HoldDownTime     int16         `json:"holdDownTime"`
	HostReach        HostReachType `json:"hostReach"`
	McastGroupL2     string        `json:"mcastGroupL2,omitempty"`
	McastGroupL3     string        `json:"mcastGroupL3,omitempty"`
	SourceInterface  string        `json:"sourceInterface"`
	SuppressARP      bool          `json:"suppressARP"`
}

func (n *NVE) IsListItem() {}

func (n *NVE) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=" + strconv.Itoa(n.ID) + "]"
}

type HostReachType string

const (
	HostReachFloodAndLearn HostReachType = "Flood_and_learn"
	HostReachBGP           HostReachType = "bgp"
)
