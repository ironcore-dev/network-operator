// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"math"
	"net/netip"
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*OSPF)(nil)

type OSPF struct {
	AdminSt  AdminSt `json:"adminSt"`
	Name     string  `json:"name"`
	DomItems struct {
		DomList []*OSPFDom `json:"Dom-list"`
	} `json:"dom-items"`
}

func (o *OSPF) IsListItem() {}

func (o *OSPF) XPath() string {
	return "System/ospf-items/inst-items/Inst-list[name=" + o.Name + "]"
}

const (
	DefaultBwRef = 40_000 // 40 Gbps
	DefaultDist  = 110
)

type OSPFDom struct {
	AdjChangeLogLevel AdjChangeLogLevel `json:"adjChangeLogLevel"`
	AdminSt           AdminSt           `json:"adminSt"`
	BwRef             int32             `json:"bwRef"`
	BwRefUnit         BwRefUnit         `json:"bwRefUnit"`
	Dist              int16             `json:"dist"`
	Name              string            `json:"name"`
	RtrID             string            `json:"rtrId"`
	IfItems           struct {
		IfList []*OSPFInterface `json:"If-list,omitempty"`
	} `json:"if-items,omitzero"`
	MaxlsapItems struct {
		Action MaxLSAAction `json:"action"`
		MaxLsa int32        `json:"maxLsa"`
	} `json:"maxlsap-items,omitzero"`
	InterleakItems struct {
		InterLeakPList []*InterLeakPList `json:"InterLeakP-list,omitempty"`
	} `json:"interleak-items,omitzero"`
	DefrtleakItems struct {
		Always string `json:"always"`
		RtMap  string `json:"rtMap"`
	} `json:"defrtleak-items,omitzero"`
}

type InterLeakPList struct {
	Asn   string      `json:"asn"`
	Inst  string      `json:"inst"`
	Proto RtLeakProto `json:"proto"`
	RtMap string      `json:"rtMap"`
}

type OSPFInterface struct {
	AdminSt              AdminSt        `json:"adminSt"`
	AdvertiseSecondaries bool           `json:"advertiseSecondaries"`
	Area                 string         `json:"area"`
	ID                   string         `json:"id"`
	NwT                  NtwType        `json:"nwT"`
	PassiveCtrl          PassiveControl `json:"passiveCtrl"`
}

type AdjChangeLogLevel string

const (
	AdjChangeLogLevelBrief  AdjChangeLogLevel = "brief"
	AdjChangeLogLevelDefail AdjChangeLogLevel = "detail"
	AdjChangeLogLevelNone   AdjChangeLogLevel = "none"
)

type BwRefUnit string

const (
	BwRefUnitMbps BwRefUnit = "mbps"
	BwRefUnitGbps BwRefUnit = "gbps"
)

type MaxLSAAction string

const (
	MaxLSAActionReject  MaxLSAAction = "reject"
	MaxLSAActionRestart MaxLSAAction = "restart"
	MaxLSAActionLog     MaxLSAAction = "log"
)

type RtLeakProto string

const (
	RtLeakProtoStatic RtLeakProto = "static"
	RtLeakProtoDirect RtLeakProto = "direct"
)

type NtwType string

const (
	NtwTypeUnspecified  NtwType = "unspecified"
	NtwTypeBroadcast    NtwType = "bcast"
	NtwTypePointToPoint NtwType = "p2p"
)

type PassiveControl string

const (
	PassiveControlUnspecified PassiveControl = "unspecified"
	PassiveControlEnabled     PassiveControl = "enabled"
	PassiveControlDisabled    PassiveControl = "disabled"
)

func isValidOSPFArea(area string) bool {
	// Try decimal integer
	if n, err := strconv.ParseUint(area, 10, 32); err == nil {
		return n <= math.MaxUint32
	}
	// Try dotted decimal using netip.Addr
	addr, err := netip.ParseAddr(area)
	return err == nil && addr.Is4()
}
