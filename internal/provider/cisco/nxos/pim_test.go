// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("pim_apr", &AnycastPeerItems{
		AcastRPPeerList: []*AnycastPeerAddr{
			{
				Addr:      "10.0.0.100/32",
				RpSetAddr: "10.0.0.2/32",
			},
			{
				Addr:      "10.0.0.100/32",
				RpSetAddr: "10.0.0.1/32",
			},
		},
	})

	Register("pim_intf", &PIMIfItems{
		IfList: []*PIMIf{
			{ID: "lo0", PimSparseMode: true},
			{ID: "lo1", PimSparseMode: true},
			{ID: "eth1/2", PimSparseMode: true},
			{ID: "eth1/1", PimSparseMode: true},
		},
	})

	rp := &StaticRP{Addr: "10.0.0.100/32"}
	rp.RpgrplistItems.RPGrpListList = []*StaticRPGrp{{GrpListName: "224.0.0.0/4"}}
	Register("pim_rp", rp)
}
