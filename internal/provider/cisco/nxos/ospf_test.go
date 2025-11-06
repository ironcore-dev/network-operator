// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strings"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

func init() {
	dom := &OSPFDom{
		Name:              DefaultVRFName,
		AdjChangeLogLevel: AdjChangeLogLevelBrief,
		AdminSt:           AdminStEnabled,
		BwRef:             40000,
		BwRefUnit:         BwRefUnitMbps,
		Dist:              110,
		RtrID:             "10.0.0.10",
	}
	dom.IfItems.IfList = make(gnmiext.List[string, *OSPFInterface])
	for _, name := range []string{"eth1/1"} {
		intf := &OSPFInterface{
			ID:                   name,
			AdminSt:              AdminStEnabled,
			AdvertiseSecondaries: true,
			Area:                 "0.0.0.0",
			NwT:                  NtwTypeUnspecified,
			PassiveCtrl:          PassiveControlUnspecified,
		}
		if strings.HasPrefix(name, "eth") {
			intf.NwT = NtwTypePointToPoint
		}
		dom.IfItems.IfList.Set(intf)
	}
	dom.MaxlsapItems.Action = MaxLSAActionReject
	dom.MaxlsapItems.MaxLsa = 12000
	dom.InterleakItems.InterLeakPList = make(gnmiext.List[InterLeakPKey, *InterLeakP])
	dom.InterleakItems.InterLeakPList.Set(&InterLeakP{InterLeakPKey: InterLeakPKey{Proto: RtLeakProtoDirect, Asn: "none", Inst: "none"}, RtMap: "REDIST-ALL"})
	dom.DefrtleakItems.Always = "no"

	ospf := &OSPF{Name: "UNDERLAY", AdminSt: AdminStEnabled}
	ospf.DomItems.DomList = make(gnmiext.List[string, *OSPFDom])
	ospf.DomItems.DomList.Set(dom)
	Register("ospf", ospf)
}
