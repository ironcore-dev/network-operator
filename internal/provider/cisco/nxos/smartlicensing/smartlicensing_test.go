// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package smartlicensing

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_SmartLicensing(t *testing.T) {
	s := &Licensing{
		URL:  "https://smartreceiver.cisco.com/licservice/license",
		CSLU: "cslu-local",
		Vrf:  "management",
	}

	got, err := s.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/licensemanager-items/inst-items/smartlicensing-items" {
		t.Errorf("expected key 'System/licensemanager-items/inst-items/smartlicensing-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems")
	}
}

func Test_CallHome(t *testing.T) {
	c := &CallHome{
		Enable:   true,
		Email:    "sch-smart-licensing@cisco.com",
		Vrf:      "management",
		Profiles: []*Profile{{Seq: 1, URL: "https://cspc-n080-ssm.wdf.sap.corp/cslu/v1/pi/CC-LA-N080-4"}},
	}

	got, err := c.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 keys, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/callhome-items/inst-items" {
		t.Errorf("expected key 'System/callhome-items/inst-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems")
	}

	update, ok = got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/callhome-items/inst-items" {
		t.Errorf("expected key 'System/callhome-items/inst-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems")
	}
}
