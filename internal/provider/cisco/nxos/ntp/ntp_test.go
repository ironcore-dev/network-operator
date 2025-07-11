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

	got, err := ntp.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 keys, got %d", len(got))
	}

	for i := range ntp.Servers {
		xpathStr := "System/time-items/prov-items/NtpProvider-list[name=" + ntp.Servers[i].Name + "]"
		update, ok := got[i].(gnmiext.EditingUpdate)
		if !ok {
			t.Errorf("expected value to be of type EditingUpdate")
		}
		if update.XPath != xpathStr {
			t.Errorf("expected xpath #%d to be %s, found %s", i, xpathStr, update.XPath)
		}
	}

	update, ok := got[2].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	ti, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_TimeItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_TimeItems")
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
}
