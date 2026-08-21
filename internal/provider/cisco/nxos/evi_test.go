// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	rtt := &Rtt{}
	rtt.Rtt, _ = RouteTarget("65000:100010") //nolint:errcheck

	rt := &RttEntry{Type: RttEntryTypeExport}
	rt.EntItems.RttEntryList.Set(rtt)

	rd, _ := RouteDistinguisher("10.0.0.10:65000") //nolint:errcheck

	evi := &BDEVI{Encap: "vxlan-100010"}
	evi.Rd = NewOption(rd)
	evi.RttpItems.RttPList.Set(rt)
	Register("evi", evi)
}
