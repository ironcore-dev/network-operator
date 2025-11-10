// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

func init() {
	vrf := &DNSVrf{Name: "management"}
	vrf.ProvItems.ProviderList = make(gnmiext.List[string, *DNSProv])
	vrf.ProvItems.ProviderList.Set(&DNSProv{Addr: "10.10.10.10"})

	prof := &DNSProf{Name: DefaultVRFName}
	prof.DomItems.Name = "example.com"
	prof.VrfItems.VrfList = make(gnmiext.List[string, *DNSVrf])
	prof.VrfItems.VrfList.Set(vrf)
	prof.ProvItems.ProviderList = make(gnmiext.List[string, *DNSProv])
	prof.ProvItems.ProviderList.Set(&DNSProv{Addr: "11.11.11.11", SrcIf: "mgmt0"})

	dns := &DNS{AdminSt: AdminStEnabled}
	dns.ProfItems.ProfList = make(gnmiext.List[string, *DNSProf])
	dns.ProfItems.ProfList.Set(prof)
	Register("dns", dns)
}
