// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	rtt := &Rtt{}
	rtt.Rtt, _ = RouteTarget("65000:100010") //nolint:errcheck

	rt := &RttEntry{Type: RttEntryTypeExport}
	rt.EntItems.RttEntryList.Set(rtt)

	evi := &BDEVI{Encap: "vxlan-100010"}
	evi.Rd, _ = RouteDistinguisher("10.0.0.10:65000") //nolint:errcheck
	evi.RttpItems.RttPList.Set(rt)
	Register("evi", evi)
}
