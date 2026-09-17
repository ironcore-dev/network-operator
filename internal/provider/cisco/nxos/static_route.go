// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

// defaultNexthopPref is the NX-OS platform default administrative distance for a
// static route next hop. It is applied when the API spec does not set a metric,
// so a repeated reconcile does not diff against the device's default.
const (
	defaultNexthopPref int32  = 1
	defaultInterface   string = "unspecified"
)

var _ gnmiext.DataElement = (*StaticRoute)(nil)

// StaticRoute represents a Route-list entry under a VRF's IPv4 routing domain.
// VRF is the routing domain name (e.g. "default") and is not serialized to JSON;
// it is only used to build the XPath.
type StaticRoute struct {
	VRF     string             `json:"-"`
	Prefix  string             `json:"prefix"`
	NhItems StaticRouteNhItems `json:"nh-items"`
}

func (*StaticRoute) IsListItem() {}

func (r *StaticRoute) XPath() string {
	return "System/ipv4-items/inst-items/dom-items/Dom-list[name=" + r.VRF +
		"]/rt-items/Route-list[prefix=" + r.Prefix + "]"
}

type StaticRouteNhItems struct {
	NexthopList gnmiext.List[StaticRouteNexthopKey, *StaticRouteNexthop] `json:"Nexthop-list"`
}

type StaticRouteNexthopKey struct {
	NhVrf  string
	NhAddr string
	NhIf   string
	Pref   int32
}

type StaticRouteNexthop struct {
	NhAddr string `json:"nhAddr"`
	NhIf   string `json:"nhIf"`
	NhVrf  string `json:"nhVrf"`
	Pref   int32  `json:"pref"`
}

func (n *StaticRouteNexthop) Key() StaticRouteNexthopKey {
	return StaticRouteNexthopKey{NhVrf: n.NhVrf, NhAddr: n.NhAddr, NhIf: n.NhIf, Pref: n.Pref}
}

func NewStaticRouteNexthop(address, vrf, intf string, metric *int32) *StaticRouteNexthop {
	pref := defaultNexthopPref
	if metric != nil {
		pref = *metric
	}

	if intf == "" {
		intf = defaultInterface
	}
	return &StaticRouteNexthop{
		NhVrf:  vrf,
		Pref:   pref,
		NhAddr: address,
		NhIf:   intf,
	}
}
