// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"
	"errors"

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
		DomAfList []*BGPDomAfItem `json:"DomAf-list"`
	} `json:"af-items"`
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

type BGPDomAfItemRetainRtt struct {
	RetainRttAll   AdminSt        `json:"retainRttAll,omitempty"`
	RetainRttRtMap Option[string] `json:"retainRttRtMap,omitempty"`
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

func (*BGPPeer) IsListItem() {}

func (p *BGPPeer) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[name=" + p.Name + "]"
}

type BGPPeerAfItem struct {
	Ctrl       Option[string] `json:"ctrl"`
	SendComExt AdminSt        `json:"sendComExt,omitempty"`
	SendComStd AdminSt        `json:"sendComStd,omitempty"`
	Type       AddressFamily  `json:"type"`
}

type BGPPeerOperItems struct {
	Addr   string `json:"addr"`
	OperSt OperSt `json:"operSt"`
}

func (p *BGPPeerOperItems) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list[addr=" + p.Addr + "]/ent-items/PeerEntry-list[addr=" + p.Addr + "]"
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
