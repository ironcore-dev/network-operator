// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*ISIS)(nil)

// ISIS represents the IS-IS routing protocol configuration on a NX-OS device.
type ISIS struct {
	AdminSt  AdminSt `json:"adminSt"`
	Name     string  `json:"name"`
	DomItems struct {
		DomList []*ISISDom `json:"Dom-list"`
	} `json:"dom-items"`
}

func (i *ISIS) IsListItem() {}

func (i *ISIS) XPath() string {
	return "System/isis-items/inst-items/Inst-list[name=" + i.Name + "]"
}

type ISISDom struct {
	IsType      ISISLevel `json:"isType"`
	Name        string    `json:"name"`
	Net         string    `json:"net"`
	PassiveDflt ISISLevel `json:"passiveDflt"`
	AfItems     struct {
		DomAfList []*ISISDomAf `json:"DomAf-list"`
	} `json:"af-items"`
	OverloadItems struct {
		AdminSt     string `json:"adminSt"`
		BgpAsNumStr string `json:"bgpAsNumStr"`
		StartupTime int    `json:"startupTime"`
		Suppress    string `json:"suppress"`
	} `json:"overload-items"`
	IfItems struct {
		IfList []*ISISInterface `json:"If-list"`
	} `json:"if-items"`
}

type ISISDomAf struct {
	Type ISISAddressFamily `json:"type"`
}

type ISISInterface struct {
	ID             string   `json:"id"`
	NetworkTypeP2P AdminSt3 `json:"networkTypeP2P"`
	V4Bfd          string   `json:"v4Bfd"`
	V4Enable       bool     `json:"v4enable"`
	V6Bfd          string   `json:"v6Bfd"`
	V6Enable       bool     `json:"v6enable"`
}

type ISISLevel string

const (
	ISISLevel1  ISISLevel = "l1"
	ISISLevel2  ISISLevel = "l2"
	ISISLevel12 ISISLevel = "l12"
)

func ISISLevelFrom(level v1alpha1.ISISLevel) ISISLevel {
	switch level {
	case v1alpha1.ISISLevel1:
		return ISISLevel1
	case v1alpha1.ISISLevel2:
		return ISISLevel2
	case v1alpha1.ISISLevel12:
		return ISISLevel12
	default:
		return ISISLevel1
	}
}

type ISISAddressFamily string

const (
	ISISAfIPv4Unicast ISISAddressFamily = "v4"
	ISISAfIPv6Unicast ISISAddressFamily = "v6"
)
