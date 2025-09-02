// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ntp

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_NTP(t *testing.T) {
	ntp := &NTP{
		EnableLogging: true,
		SrcInterface:  "mgmt0",
		Servers: []*Server{
			{
				Name:      "192.168.0.1",
				Preferred: true,
				Vrf:       "CC-MGMT",
			},
			{
				Name:      "192.168.0.2",
				Preferred: true,
				Vrf:       "CC-MGMT",
			},
		},
	}

	got, err := ntp.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 updates, got %d", len(got))
	}

	ntpdUpdate, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected first update to be of type EditingUpdate")
	}

	expectedNtpdXPath := "System/fm-items/ntpd-items"
	if ntpdUpdate.XPath != expectedNtpdXPath {
		t.Errorf("expected first update xpath to be %s, found %s", expectedNtpdXPath, ntpdUpdate.XPath)
	}

	ntpdItems, ok := ntpdUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_NtpdItems)
	if !ok {
		t.Errorf("expected first update value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_NtpdItems")
	}

	if ntpdItems.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected NTP daemon AdminSt to be enabled, got %v", ntpdItems.AdminSt)
	}

	// Test second update - time items configuration
	timeUpdate, ok := got[1].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected second update to be of type ReplacingUpdate")
	}

	expectedTimeXPath := "System/time-items"
	if timeUpdate.XPath != expectedTimeXPath {
		t.Errorf("expected second update xpath to be %s, found %s", expectedTimeXPath, timeUpdate.XPath)
	}

	ti, ok := timeUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_TimeItems)
	if !ok {
		t.Errorf("expected second update value to be of type *nxos.Cisco_NX_OSDevice_System_TimeItems")
	}

	if ti.Logging == nxos.Cisco_NX_OSDevice_Datetime_AdminState_UNSET {
		t.Errorf("expected value for 'System/time-items/logging' to be present")
	}
	if ti.SrcIfItems == nil || ti.SrcIfItems.SrcIf == nil {
		t.Errorf("expected value for 'System/time-items/srcIf-items' to be present")
	}
	if ti.ProvItems == nil || len(ti.ProvItems.NtpProviderList) != len(ntp.Servers) {
		t.Errorf("expected %d NTP servers in 'System/time-items/prov-items'", len(ntp.Servers))
	}

	// Verify server configurations
	for _, server := range ntp.Servers {
		ntpProvider, exists := ti.ProvItems.NtpProviderList[server.Name]
		if !exists {
			t.Errorf("expected NTP server %s to be present in provider list", server.Name)
			continue
		}
		if ntpProvider.Name == nil || *ntpProvider.Name != server.Name {
			t.Errorf("expected NTP server name to be %s, got %v", server.Name, ntpProvider.Name)
		}
		if ntpProvider.Preferred == nil || *ntpProvider.Preferred != server.Preferred {
			t.Errorf("expected NTP server %s preferred to be %t, got %v", server.Name, server.Preferred, ntpProvider.Preferred)
		}
		if ntpProvider.Vrf == nil || *ntpProvider.Vrf != server.Vrf {
			t.Errorf("expected NTP server %s VRF to be %s, got %v", server.Name, server.Vrf, ntpProvider.Vrf)
		}
	}
}
