// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var _ gnmiext.Configurable = (*DNS)(nil)

// DNS represents the DNS configuration on a NX-OS device.
type DNS struct {
	AdminSt   AdminSt `json:"adminSt"`
	ProfItems struct {
		ProfList []*DNSProf `json:"Prof-list,omitzero"`
	} `json:"prof-items,omitzero"`
}

func (*DNS) XPath() string {
	return "System/dns-items"
}

type DNSProf struct {
	Name      string `json:"name"`
	ProvItems struct {
		ProviderList []*DNSProv `json:"Provider-list,omitzero"`
	} `json:"prov-items,omitzero"`
	VrfItems struct {
		VrfList []*DNSVrf `json:"Vrf-list,omitzero"`
	} `json:"vrf-items,omitzero"`
	DomItems struct {
		Name string `json:"name,omitempty"`
	} `json:"dom-items,omitzero"`
}

type DNSVrf struct {
	Name      string `json:"name"`
	ProvItems struct {
		ProviderList []*DNSProv `json:"Provider-list,omitzero"`
	} `json:"prov-items,omitzero"`
}

type DNSProv struct {
	Addr  string `json:"addr"`
	SrcIf string `json:"srcIf,omitempty"`
}
