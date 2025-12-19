// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*MultisiteItems)(nil)

type MultisiteItems struct {
	SiteID              string  `json:"siteId"`
	AdminSt             AdminSt `json:"state"`
	DelayRestoreSeconds int64   `json:"delayRestoreTime"`
}

func (*MultisiteItems) XPath() string {
	return "System/eps-items/multisite-items"
}

type MultisiteBorderGatewayInterface string

func (*MultisiteBorderGatewayInterface) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]/multisiteBordergwInterface"
}

type StormControlItems struct {
	EvpnStormControlList gnmiext.List[StormControlType, *StormControlItem] `json:"EvpnStormControl-list"`
}

func (*StormControlItems) XPath() string {
	return "System/eps-items/stormcontrol-items"
}

type StormControlItem struct {
	Floatlevel string           `json:"floatlevel"`
	Name       StormControlType `json:"name"`
}

func (ctrl *StormControlItem) Key() StormControlType { return ctrl.Name }

type StormControlType string

const (
	StormControlTypeUnicast   StormControlType = "ucast"
	StormControlTypeMulticast StormControlType = "mcast"
	StormControlTypeBroadcast StormControlType = "bcast"
)
