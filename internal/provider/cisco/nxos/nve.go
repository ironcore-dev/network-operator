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
	AdminSt          AdminSt        `json:"adminSt"`
	AdvertiseVmac    bool           `json:"advertiseVmac"`
	AnycastInterface Option[string] `json:"anycastIntf"`
	ID               int            `json:"epId"`
	HoldDownTime     int16          `json:"holdDownTime"`
	HostReach        HostReachType  `json:"hostReach"`
	McastGroupL2     Option[string] `json:"mcastGroupL2"`
	McastGroupL3     Option[string] `json:"mcastGroupL3"`
	SourceInterface  string         `json:"sourceInterface"`
	SuppressARP      bool           `json:"suppressARP"`
}

func (*NVE) IsListItem() {}

func (n *NVE) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=" + strconv.Itoa(n.ID) + "]"
}

type VNI struct {
	AssociateVrfFlag bool           `json:"associateVrfFlag"`
	McastGroup       Option[string] `json:"mcastGroup"`
	Vni              int32          `json:"vni"`
}

func (*VNI) IsListItem() {}

func (v *VNI) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]/nws-items/vni-items/Nw-list[vni=" + strconv.FormatInt(int64(v.Vni), 10) + "]"
}

type VNIOperItems struct {
	Vni   int32  `json:"vni"`
	State OperSt `json:"state"`
}

func (v *VNIOperItems) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]/nws-items/opervni-items/OperNw-list[vni=" + strconv.FormatInt(int64(v.Vni), 10) + "]"
}

type HostReachType string

const (
	HostReachFloodAndLearn HostReachType = "Flood_and_learn"
	HostReachBGP           HostReachType = "bgp"
)

type VNIState string

const (
	VNIStateUp   VNIState = "Up"
	VNIStateDown VNIState = "Down"
)
