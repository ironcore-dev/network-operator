// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*BGP)(nil)
	_ gnmiext.Configurable = (*BGPDom)(nil)
)

type BGP struct {
	AdminSt  AdminSt `json:"adminSt"`
	Asn      string  `json:"asn"`
	DomItems struct {
		DomList []*BGPDom `json:"Dom-list"`
	} `json:"dom-items"`
}

func (*BGP) XPath() string {
	return "System/bgp-items/inst-items"
}

type BGPDom struct {
	Name      string  `json:"name"`
	RtrID     string  `json:"rtrId"`
	RtrIDAuto AdminSt `json:"rtrIdAuto"`
	AfItems   struct {
		DomAfList []*BGPDomAfItem `json:"DomAf-list"`
	} `json:"af-items"`
}

func (d *BGPDom) IsListItem() {}

func (d *BGPDom) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + d.Name + "]"
}

type BGPDomAfItem struct {
	MaxExtEcmp     uint8         `json:"maxExtEcmp,omitempty"`
	RetainRttAll   AdminSt       `json:"retainRttAll,omitempty"`
	RetainRttRtMap string        `json:"retainRttRtMap,omitempty"`
	Type           AddressFamily `json:"type"`
}

type BGPPeer struct {
	Addr    string      `json:"addr"`
	Asn     string      `json:"asn"`
	AsnType PeerAsnType `json:"asnType"`
	Name    string      `json:"name"`
	SrcIf   string      `json:"srcIf"`
	AfItems struct {
		PeerAfList []*BGPPeerAfItem `json:"PeerAf-list"`
	} `json:"af-items"`
}

func (p *BGPPeer) IsListItem() {}

func (p *BGPPeer) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[name=" + p.Name + "]"
}

type BGPPeerAfItem struct {
	Ctrl       Option[string] `json:"ctrl"`
	SendComExt AdminSt        `json:"sendComExt,omitempty"`
	SendComStd AdminSt        `json:"sendComStd,omitempty"`
	Type       AddressFamily  `json:"type"`
}

type PeerAsnType string

const (
	PeerAsnTypeNone     PeerAsnType = "none"
	PeerAsnTypeExternal PeerAsnType = "external"
	PeerAsnTypeInternal PeerAsnType = "internal"
)

const RouteReflectorClient = "rr-client"
