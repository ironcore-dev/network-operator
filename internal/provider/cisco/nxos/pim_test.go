// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	apItems := new(AnycastPeerItems)
	apItems.AcastRPPeerList.Set(&AnycastPeerAddr{Addr: "10.0.0.100/32", RpSetAddr: "10.0.0.2/32"})
	Register("pim_apr", apItems)

	ifItems := new(PIMIfItems)
	ifItems.IfList.Set(&PIMIf{ID: "eth1/1", PimSparseMode: true})
	Register("pim_intf", ifItems)

	rp := &StaticRP{Addr: "10.0.0.100/32"}
	rp.RpgrplistItems.RPGrpListList.Set(&StaticRPGrp{GrpListName: "224.0.0.0/4"})
	Register("pim_rp", rp)
}
