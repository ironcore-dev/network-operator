// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	vrf := &DNSVrf{Name: "management"}
	vrf.ProvItems.ProviderList = []*DNSProv{{Addr: "10.10.10.10"}}

	prof := &DNSProf{Name: DefaultVRFName}
	prof.DomItems.Name = "example.com"
	prof.VrfItems.VrfList = []*DNSVrf{vrf}
	prof.ProvItems.ProviderList = []*DNSProv{{Addr: "11.11.11.11", SrcIf: "mgmt0"}}

	dns := &DNS{AdminSt: AdminStEnabled}
	dns.ProfItems.ProfList = []*DNSProf{prof}
	Register("dns", dns)
}
