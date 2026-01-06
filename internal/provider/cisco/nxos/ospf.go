// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"time"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*OSPF)(nil)

type OSPF struct {
	AdminSt  AdminSt `json:"adminSt"`
	Name     string  `json:"name"`
	DomItems struct {
		DomList gnmiext.List[string, *OSPFDom] `json:"Dom-list,omitzero"`
	} `json:"dom-items,omitzero"`
}

func (*OSPF) IsListItem() {}

func (o *OSPF) XPath() string {
	return "System/ospf-items/inst-items/Inst-list[name=" + o.Name + "]"
}

const (
	DefaultBwRef = 40_000 // 40 Gbps
	DefaultDist  = 110
)

var _ gnmiext.Keyed[string] = (*OSPFDom)(nil)

type OSPFDom struct {
	AdjChangeLogLevel AdjChangeLogLevel `json:"adjChangeLogLevel"`
	AdminSt           AdminSt           `json:"adminSt"`
	BwRef             int32             `json:"bwRef"`
	BwRefUnit         BwRefUnit         `json:"bwRefUnit"`
	Ctrl              string            `json:"ctrl,omitempty"`
	Dist              int16             `json:"dist"`
	Name              string            `json:"name"`
	RtrID             string            `json:"rtrId"`
	IfItems           struct {
		IfList gnmiext.List[string, *OSPFInterface] `json:"If-list,omitzero"`
	} `json:"if-items,omitzero"`
	MaxlsapItems struct {
		Action MaxLSAAction `json:"action"`
		MaxLsa int32        `json:"maxLsa"`
	} `json:"maxlsap-items,omitzero"`
	InterleakItems struct {
		InterLeakPList gnmiext.List[InterLeakPKey, *InterLeakP] `json:"InterLeakP-list,omitzero"`
	} `json:"interleak-items,omitzero"`
	DefrtleakItems struct {
		Always string `json:"always"`
		RtMap  string `json:"rtMap"`
	} `json:"defrtleak-items,omitzero"`
}

func (o *OSPFDom) Key() string { return o.Name }

type InterLeakPKey struct {
	Asn   string      `json:"asn"`
	Inst  string      `json:"inst"`
	Proto RtLeakProto `json:"proto"`
}

type InterLeakP struct {
	InterLeakPKey

	RtMap string `json:"rtMap"`
}

func (o *InterLeakP) Key() InterLeakPKey { return o.InterLeakPKey }

type OSPFInterface struct {
	AdminSt              AdminSt        `json:"adminSt"`
	AdvertiseSecondaries bool           `json:"advertiseSecondaries"`
	Area                 string         `json:"area"`
	ID                   string         `json:"id"`
	NwT                  NtwType        `json:"nwT"`
	PassiveCtrl          PassiveControl `json:"passiveCtrl"`
}

func (i *OSPFInterface) Key() string { return i.ID }

type OSPFOperItems struct {
	Name    string `json:"name"`
	OperSt  OperSt `json:"operSt"`
	IfItems struct {
		IfList []*OSPFIfOperItems `json:"If-list"`
	} `json:"if-items"`
}

func (*OSPFOperItems) IsListItem() {}

func (o *OSPFOperItems) XPath() string {
	return "System/ospf-items/inst-items/Inst-list[name=" + o.Name + "]/dom-items/Dom-list[name=" + DefaultVRFName + "]"
}

type OSPFIfOperItems struct {
	ID       string `json:"id"`
	AdjItems struct {
		AdjList []*OSPFIfAdjEpGroup `json:"AdjEp-list,omitzero"`
	} `json:"adj-items,omitzero"`
}

type OSPFIfAdjEpGroup struct {
	ID            string    `json:"id"`     // Adjacency neighbor's router id
	PeerIP        string    `json:"peerIp"` // Adjacency neighbor's ip address
	OperSt        AdjOperSt `json:"operSt"` // Adjacency neighbor state
	Prio          uint8     `json:"prio"`   // Priority, used in determining the designated router on this network
	AdjStatsItems struct {
		LastStChgTs time.Time `json:"lastStChgTs"` // Timestamp of the last state change
	} `json:"adjstats-items,omitzero"`
}

type AdjOperSt string

const (
	AdjOperStUnknown      AdjOperSt = "unknown"
	AdjOperStDown         AdjOperSt = "down"
	AdjOperStAttempt      AdjOperSt = "attempt"
	AdjOperStInitializing AdjOperSt = "initializing"
	AdjOperStTwoWay       AdjOperSt = "two-way"
	AdjOperStExstart      AdjOperSt = "exstart"
	AdjOperStExchange     AdjOperSt = "exchange"
	AdjOperStLoading      AdjOperSt = "loading"
	AdjOperStFull         AdjOperSt = "full"
	AdjOperStSelf         AdjOperSt = "self"
)

func (s AdjOperSt) ToNeighborState() v1alpha1.OSPFNeighborState {
	switch s {
	case AdjOperStDown:
		return v1alpha1.OSPFNeighborStateDown
	case AdjOperStAttempt:
		return v1alpha1.OSPFNeighborStateAttempt
	case AdjOperStInitializing:
		return v1alpha1.OSPFNeighborStateInit
	case AdjOperStTwoWay:
		return v1alpha1.OSPFNeighborStateTwoWay
	case AdjOperStExstart:
		return v1alpha1.OSPFNeighborStateExStart
	case AdjOperStExchange:
		return v1alpha1.OSPFNeighborStateExchange
	case AdjOperStLoading:
		return v1alpha1.OSPFNeighborStateLoading
	case AdjOperStFull:
		return v1alpha1.OSPFNeighborStateFull
	case AdjOperStSelf:
		// Not defined in RFC2328
		fallthrough
	default:
		return v1alpha1.OSPFNeighborStateUnknown
	}
}

type AdjChangeLogLevel string

const (
	AdjChangeLogLevelBrief  AdjChangeLogLevel = "brief"
	AdjChangeLogLevelDetail AdjChangeLogLevel = "detail"
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
