// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*BGP)(nil)
	_ gnmiext.DataElement = (*BGPDom)(nil)
	_ gnmiext.DataElement = (*BGPDomItems)(nil)
	_ gnmiext.DataElement = (*BGPPeerGroup)(nil)
)

// ownershipMarkerPrefix is used to build per-VRF peer template names written
// into the default VRF domain. Each marker identifies an operator-managed BGP
// domain by its VRF name, and is used during deletion to decide whether the
// global BGP instance can be cleaned up.
const ownershipMarkerPrefix = "__operator-managed--"

func ownershipMarkerName(vrfName string) string {
	return ownershipMarkerPrefix + vrfName + "__"
}

func isOwnershipMarker(name string) bool {
	return strings.HasPrefix(name, ownershipMarkerPrefix)
}

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
	PeerContItems struct {
		PeerContList gnmiext.List[string, *BGPPeerGroup] `json:"PeerCont-list,omitzero"`
	} `json:"peercont-items,omitzero"`
}

func (*BGPDom) IsListItem() {}

func (d *BGPDom) Key() string { return d.Name }

func (d *BGPDom) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + d.Name + "]"
}

// BGPDomItems is the container for all BGP domains configured on the device.
type BGPDomItems struct {
	DomList gnmiext.List[string, *BGPDom] `json:"Dom-list,omitzero"`
}

func (*BGPDomItems) XPath() string {
	return "System/bgp-items/inst-items/dom-items"
}

// BGPPeerGroup is a template peer group under a BGP domain.
type BGPPeerGroup struct {
	VRFName string `json:"-"`
	Name    string `json:"name"`
}

func (*BGPPeerGroup) IsListItem() {}

func (g *BGPPeerGroup) Key() string { return g.Name }

func (g *BGPPeerGroup) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + g.VRFName + "]/peercont-items/PeerCont-list[name=" + g.Name + "]"
}

type BGPDomAfItem struct {
	MaxExtEcmp    int8          `json:"maxExtEcmp,omitempty"`
	MaxExtIntEcmp int8          `json:"maxExtIntEcmp,omitempty"`
	ExportGwIP    AdminSt       `json:"exportGwIp"`
	Type          AddressFamily `json:"type"`

	// The fields below are only valid for the l2vpn-evpn address family.
	// For other address families, these fields will be omitted in the JSON
	// representation.
	AdvPip         AdminSt        `json:"advPip,omitempty"`
	RetainRttAll   AdminSt        `json:"retainRttAll,omitempty"`
	RetainRttRtMap Option[string] `json:"retainRttRtMap"`

	InterLeakPItems struct {
		InterLeakPList gnmiext.List[InterLeakPKey, *InterLeakP] `json:"InterLeakP-list,omitzero"`
	} `json:"interleak-items,omitzero"`
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
			MaxExtEcmp      int8          `json:"maxExtEcmp,omitempty"`
			MaxExtIntEcmp   int8          `json:"maxExtIntEcmp,omitempty"`
			ExportGwIP      AdminSt       `json:"exportGwIp"`
			Type            AddressFamily `json:"type"`
			InterLeakPItems struct {
				InterLeakPList gnmiext.List[InterLeakPKey, *InterLeakP] `json:"InterLeakP-list,omitzero"`
			} `json:"interleak-items,omitzero"`
		}{
			MaxExtEcmp:      af.MaxExtEcmp,
			MaxExtIntEcmp:   af.MaxExtIntEcmp,
			ExportGwIP:      af.ExportGwIP,
			Type:            af.Type,
			InterLeakPItems: af.InterLeakPItems,
		})
	}
	// ExportGwIP is not valid for l2vpn-evpn; set it to disabled.
	cpy.ExportGwIP = AdminStDisabled
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
		af.AdvPip = ""
		af.RetainRttAll = ""
		af.RetainRttRtMap = Option[string]{}
	}
	return nil
}

func (af *BGPDomAfItem) Key() AddressFamily { return af.Type }

// NewInterLeakPDirect creates an InterLeakP entry for redistributing directly
// connected routes into a BGP address family.
func NewInterLeakPDirect(rtMap string) *InterLeakP {
	return &InterLeakP{
		InterLeakPKey: InterLeakPKey{
			Asn:   "none",
			Inst:  "none",
			Proto: RtLeakProtoDirect,
		},
		RtMap: rtMap,
	}
}

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
	VRFName       string      `json:"-"`
	Addr          string      `json:"addr"`
	AdminSt       AdminSt     `json:"adminSt"`
	Asn           string      `json:"asn"`
	AsnType       PeerAsnType `json:"asnType"`
	Name          string      `json:"name,omitempty"`
	SrcIf         string      `json:"srcIf,omitempty"`
	LocalAsnItems struct {
		AsnPropagate AsnPropagate `json:"asnPropagate"`
		LocalAsn     string       `json:"localAsn"`
	} `json:"localasn-items,omitzero"`
	AfItems struct {
		PeerAfList gnmiext.List[AddressFamily, *BGPPeerAfItem] `json:"PeerAf-list,omitzero"`
	} `json:"af-items,omitzero"`
}

type AsnPropagate string

const (
	// AsnPropagateNone sends the local-as with no additional options.
	AsnPropagateNone AsnPropagate = "none"
	// AsnPropagateNoPrep does not prepend the local-as number to updates
	// received from the eBGP neighbor.
	AsnPropagateNoPrep AsnPropagate = "no-prepend"
	// AsnPropagateReplaceAs prepends only the local-as number to updates
	// sent to the eBGP neighbor.
	AsnPropagateReplaceAs AsnPropagate = "replace-as"
	// AsnPropagateDualAs allows the peer to connect using either the
	// local-as number or the real AS.
	AsnPropagateDualAs AsnPropagate = "dual-as"
)

func (*BGPPeer) IsListItem() {}

func (p *BGPPeer) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + p.VRFName + "]/peer-items/Peer-list[addr=" + p.Addr + "]"
}

type BGPPeerAfItem struct {
	Ctrl       Option[string] `json:"ctrl"`
	SendComExt AdminSt        `json:"sendComExt"`
	SendComStd AdminSt        `json:"sendComStd"`
	Type       AddressFamily  `json:"type"`

	RtCtrlPItems struct {
		RtCtrlPList gnmiext.List[RtCtrlDirection, *BGPPeerAfRtCtrlP] `json:"RtCtrlP-list,omitzero"`
	} `json:"rtctrl-items,omitzero"`
}

func (af *BGPPeerAfItem) Key() AddressFamily { return af.Type }

type RtCtrlDirection string

const (
	RtCtrlDirectionIn  RtCtrlDirection = "in"
	RtCtrlDirectionOut RtCtrlDirection = "out"
)

type BGPPeerAfRtCtrlP struct {
	Direction RtCtrlDirection `json:"direction"`
	RtMap     string          `json:"rtMap"`
}

func (r *BGPPeerAfRtCtrlP) Key() RtCtrlDirection { return r.Direction }

type BGPPeerOperItems struct {
	VRFName      string        `json:"-"`
	Addr         string        `json:"addr"`
	OperSt       BGPPeerOperSt `json:"operSt"`
	LastFlapTime time.Time     `json:"lastFlapTs"`
	AfItems      struct {
		PeerAfList []*BGPPeerAfOperItems `json:"PeerAfEntry-list,omitempty"`
	} `json:"af-items,omitzero"`
}

func (*BGPPeerOperItems) IsListItem() {}

func (p *BGPPeerOperItems) XPath() string {
	return "System/bgp-items/inst-items/dom-items/Dom-list[name=" + p.VRFName + "]/peer-items/Peer-list[addr=" + p.Addr + "]/ent-items/PeerEntry-list[addr=" + p.Addr + "]"
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
	AsFormatAsDot AsFormat = "asdot"
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
