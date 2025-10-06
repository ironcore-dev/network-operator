// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "strings"

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
	for _, name := range []string{"eth1/1", "lo1", "eth1/2", "lo0"} {
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
		dom.IfItems.IfList = append(dom.IfItems.IfList, intf)
	}
	dom.MaxlsapItems.Action = MaxLSAActionReject
	dom.MaxlsapItems.MaxLsa = 12000
	dom.InterleakItems.InterLeakPList = []*InterLeakPList{{Proto: RtLeakProtoDirect, Asn: "none", Inst: "none", RtMap: "REDIST-ALL"}}
	dom.DefrtleakItems.Always = "no"
	ospf := &OSPF{Name: "UNDERLAY", AdminSt: AdminStEnabled}
	ospf.DomItems.DomList = []*OSPFDom{dom}
	Register("ospf", ospf)
}
