// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_DNS_Enable(t *testing.T) {
	d := &DNS{
		DomainName: "sap.corp",
		Providers: []*Provider{
			{
				Addr: "147.204.8.200",
				Vrf:  "mgmt0",
			},
			{
				Addr: "147.204.8.201",
				Vrf:  "mgmt0",
			},
			{
				Addr: "147.204.8.200",
				Vrf:  "lo1",
			},
			{
				Addr: "147.204.8.201",
				Vrf:  "lo1",
			},
		},
		Enable: true,
	}

	got, err := d.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/dns-items" {
		t.Errorf("expected key 'System/dns-items' to be present")
	}
	dns, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_DnsItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_DnsItems")
	}
	if dns.AdminSt != nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled {
		t.Errorf("expected AdminSt to be 'enabled', got '%v'", dns.AdminSt)
	}

	profList, ok := dns.ProfItems.ProfList["default"]
	if !ok {
		t.Errorf("expected key 'default' in ProfList to be present")
	}
	if profList.Name == nil || *profList.Name != "default" {
		t.Errorf("expected ProfList name to be 'default', got '%v'", profList.Name)
	}

	if profList.DomItems == nil || profList.DomItems.Name == nil || *profList.DomItems.Name != d.DomainName {
		t.Errorf("expected DomainName to be '%s', got '%v'", d.DomainName, profList.DomItems.Name)
	}

	for _, s := range d.Providers {
		vrfList, ok := profList.VrfItems.VrfList[s.Vrf]
		if !ok {
			t.Errorf("expected key '%s' in VrfList to be present", s.Vrf)
		}
		if vrfList.Name == nil || *vrfList.Name != s.Vrf {
			t.Errorf("expected VrfList name to be '%s', got '%v'", s.Vrf, vrfList.Name)
		}
		provider, ok := vrfList.ProvItems.ProviderList[s.Addr]
		if !ok {
			t.Errorf("expected key '%s' in ProviderList to be present", s.Addr)
		}
		if provider.Addr == nil || *provider.Addr != s.Addr {
			t.Errorf("expected ProviderList addr to be '%s', got '%v'", s.Addr, provider.Addr)
		}
	}
}

func Test_DNS_Disable(t *testing.T) {
	d := &DNS{
		DomainName: "sap.corp",
		Providers: []*Provider{
			{
				Addr: "147.204.8.200",
				Vrf:  "mgmt0",
			},
		},
		Enable: false,
	}

	got, err := d.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, updates := range got {
		update, ok := updates.(gnmiext.EditingUpdate)
		if !ok {
			t.Errorf("expected value to be of type EditingUpdate")
		}
		if err := ygot.ValidateGoStruct(update.Value); err != nil {
			t.Errorf("ygot struct validation failed")
		}
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/dns-items" {
		t.Errorf("expected key 'System/dns-items' to be present")
	}
	dns, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_DnsItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_DnsItems")
	}
	if dns.AdminSt != nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled {
		t.Errorf("expected AdminSt to be 'disabled', got '%v'", dns.AdminSt)
	}

	if dns.ProfItems != nil {
		t.Errorf("expected ProfItems to be nil, got '%v'", dns.ProfItems)
	}
}
