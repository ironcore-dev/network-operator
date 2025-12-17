// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"
	"errors"
	"time"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*BGP)(nil)
	_ gnmiext.Configurable = (*BGPDom)(nil)
)

type BGP struct {
	AdminSt AdminSt `json:"adminSt"`
	Asn     string  `json:"asn"`
}

func (*BGP) XPath() string {
	return "System/bgp-items/inst-items"
}

type BGPDom struct {
	Name      string  `json:"name"`
	RtrID     string  `json:"rtrId"`
	RtrIDAuto AdminSt `json:"rtrIdAuto"`
	AfItems   struct {
		DomAfList gnmiext.List[AddressFamily, *BGPDomAfItem] `json:"DomAf-list,omitzero"`
	} `json:"af-items,omitzero"`
}

func (*BGPDom) IsListItem() {}

func (d *BGPDom) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + d.Name + "]"
}

type BGPDomAfItem struct {
	MaxExtEcmp    int8          `json:"maxExtEcmp,omitempty"`
	MaxExtIntEcmp int8          `json:"maxExtIntEcmp,omitempty"`
	Type          AddressFamily `json:"type"`

	// The fields retainRttAll and retainRttRtMap are only valid for the l2vpn-evpn
	// address family. For other address families, these fields will be omitted
	// in the JSON representation.
	RetainRttAll   AdminSt        `json:"retainRttAll,omitempty"`
	RetainRttRtMap Option[string] `json:"retainRttRtMap"`
}

var (
	_ json.Marshaler   = BGPDomAfItem{}
	_ json.Unmarshaler = (*BGPDomAfItem)(nil)
)

func (af BGPDomAfItem) MarshalJSON() ([]byte, error) {
	// Create a new type to avoid infinite recursion
	type Copy BGPDomAfItem
	cpy := Copy(af)
	if af.Type != AddressFamilyL2EVPN {
		return json.Marshal(struct {
			MaxExtEcmp    int8          `json:"maxExtEcmp,omitempty"`
			MaxExtIntEcmp int8          `json:"maxExtIntEcmp,omitempty"`
			Type          AddressFamily `json:"type"`
		}{
			MaxExtEcmp:    af.MaxExtEcmp,
			MaxExtIntEcmp: af.MaxExtIntEcmp,
			Type:          af.Type,
		})
	}
	return json.Marshal(cpy)
}

func (af *BGPDomAfItem) UnmarshalJSON(v []byte) error {
	// Create a new type to avoid infinite recursion
	type Copy BGPDomAfItem
	var cpy Copy
	if err := json.Unmarshal(v, &cpy); err != nil {
		return err
	}
	*af = BGPDomAfItem(cpy)
	if af.Type != AddressFamilyL2EVPN {
		af.RetainRttAll = ""
		af.RetainRttRtMap = Option[string]{}
	}
	return nil
}

func (af *BGPDomAfItem) Key() AddressFamily { return af.Type }

func (af *BGPDomAfItem) SetMultipath(m *v1alpha1.BGPMultipath) error {
	// Default from YANG model
	af.MaxExtEcmp = 1
	af.MaxExtIntEcmp = 1
	if m == nil || !m.Enabled {
		return nil
	}
	if m.Ebgp != nil {
		af.MaxExtEcmp = m.Ebgp.MaximumPaths
		if m.Ebgp.AllowMultipleAs {
			return errors.New("allowing multiple AS numbers for eBGP multipath is not supported on Cisco NX-OS")
		}
	}
	if m.Ibgp != nil {
		af.MaxExtIntEcmp = m.Ibgp.MaximumPaths
	}
	return nil
}

type BGPPeer struct {
	Addr                string      `json:"addr"`
	AdminSt             AdminSt     `json:"adminSt"`
	Asn                 string      `json:"asn"`
	AsnType             PeerAsnType `json:"asnType"`
	Name                string      `json:"name,omitempty"`
	SrcIf               string      `json:"srcIf,omitempty"`
	InheritContPeerCtrl string      `json:"inheritContPeerCtrl"`
	AfItems             struct {
		PeerAfList gnmiext.List[AddressFamily, *BGPPeerAfItem] `json:"PeerAf-list,omitzero"`
	} `json:"af-items,omitzero"`
}

func (*BGPPeer) IsListItem() {}

func (p *BGPPeer) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr + "]"
}

type BGPPeerAfItem struct {
	Ctrl       Option[string] `json:"ctrl"`
	SendComExt AdminSt        `json:"sendComExt"`
	SendComStd AdminSt        `json:"sendComStd"`
	Type       AddressFamily  `json:"type"`
}

func (af *BGPPeerAfItem) Key() AddressFamily { return af.Type }

type BGPPeerOperItems struct {
	Addr         string        `json:"addr"`
	OperSt       BGPPeerOperSt `json:"operSt"`
	LastFlapTime time.Time     `json:"lastFlapTs"`
	AfItems      struct {
		PeerAfList []*BGPPeerAfOperItems `json:"PeerAfEntry-list,omitempty"`
	} `json:"af-items,omitzero"`
}

func (*BGPPeerOperItems) IsListItem() {}

func (p *BGPPeerOperItems) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr + "]/ent-items/PeerEntry-list[addr=" + p.Addr + "]"
}

type BGPPeerAfOperItems struct {
	AcceptedPaths uint32        `json:"acceptedPaths"`
	PfxSent       string        `json:"pfxSent"`
	Type          AddressFamily `json:"type"`
}

type BGPPeerOperSt string

const (
	BGPPeerOperStIdle        BGPPeerOperSt = "idle"
	BGPPeerOperStConnect     BGPPeerOperSt = "connect"
	BGPPeerOperStActive      BGPPeerOperSt = "active"
	BGPPeerOperStOpenSent    BGPPeerOperSt = "opensent"
	BGPPeerOperStOpenConfirm BGPPeerOperSt = "openconfirm"
	BGPPeerOperStEstablished BGPPeerOperSt = "established"
)

func (s BGPPeerOperSt) ToSessionState() v1alpha1.BGPPeerSessionState {
	switch s {
	case BGPPeerOperStIdle:
		return v1alpha1.BGPPeerSessionStateIdle
	case BGPPeerOperStConnect:
		return v1alpha1.BGPPeerSessionStateConnect
	case BGPPeerOperStActive:
		return v1alpha1.BGPPeerSessionStateActive
	case BGPPeerOperStOpenSent:
		return v1alpha1.BGPPeerSessionStateOpenSent
	case BGPPeerOperStOpenConfirm:
		return v1alpha1.BGPPeerSessionStateOpenConfirm
	case BGPPeerOperStEstablished:
		return v1alpha1.BGPPeerSessionStateEstablished
	default:
		return v1alpha1.BGPPeerSessionStateUnknown
	}
}

type MultisitePeerItems struct {
	PeerList []struct {
		Addr     string                `json:"addr"`
		PeerType BorderGatewayPeerType `json:"peerType"`
	} `json:"Peer-list"`
}

func (*MultisitePeerItems) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items"
}

var (
	_ json.Marshaler   = MultisitePeer{}
	_ json.Unmarshaler = (*MultisitePeer)(nil)
)

type MultisitePeer struct {
	Addr     string                `json:"-"`
	PeerType BorderGatewayPeerType `json:"-"`
}

func (p *MultisitePeer) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr + "]/peerType"
}

func (p MultisitePeer) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.PeerType)
}

func (p *MultisitePeer) UnmarshalJSON(b []byte) error {
	var t string
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	p.PeerType = BorderGatewayPeerType(t)
	return nil
}

type AsFormat string

func (AsFormat) XPath() string {
	return "System/l3vm-items/asFormat"
}

const (
	AsFormatAsDot AsFormat = "as-dot"
)

type PeerAsnType string

const (
	PeerAsnTypeNone     PeerAsnType = "none"
	PeerAsnTypeExternal PeerAsnType = "external"
	PeerAsnTypeInternal PeerAsnType = "internal"
)

const RouteReflectorClient = "rr-client"

type BorderGatewayPeerType string

const (
	BorderGatewayPeerTypeFabricExternal   BorderGatewayPeerType = "fabric-external"
	BorderGatewayPeerTypeFabricBorderLeaf BorderGatewayPeerType = "fabric-border-leaf"
)

func BorderGatewayPeerTypeFrom(t nxv1alpha1.BGPPeerType) BorderGatewayPeerType {
	switch t {
	case nxv1alpha1.BGPPeerTypeFabricExternal:
		return BorderGatewayPeerTypeFabricExternal
	case nxv1alpha1.BGPPeerTypeFabricBorderLeaf:
		return BorderGatewayPeerTypeFabricBorderLeaf
	default:
		return BorderGatewayPeerTypeFabricExternal
	}
}
