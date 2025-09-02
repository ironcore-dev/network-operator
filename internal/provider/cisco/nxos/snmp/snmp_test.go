// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package snmp

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var mockedClient = &gnmiext.ClientMock{
	// mock a get method that populates the SNMP structure with an admin user and shutdown items
	GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct) error {
		snmpItems, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems)
		if !ok {
			return fmt.Errorf("expected *nxos.Cisco_NX_OSDevice_System_SnmpItems, got %T", dest)
		}

		adminUser := snmpItems.GetOrCreateInstItems().GetOrCreateLclUserItems().GetOrCreateLocalUserList("admin")
		*adminUser = *createAdminUser() // copy the admin user structure

		shutdown := snmpItems.GetOrCreateServershutdownItems()
		*shutdown = *createShutdownItems() // copy the shutdown items structure
		return nil
	},
}

func createAdminUser() *nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList {
	snmpItems := nxos.Cisco_NX_OSDevice_System_SnmpItems{}
	adminUser := snmpItems.GetOrCreateInstItems().GetOrCreateLclUserItems().GetOrCreateLocalUserList("admin")
	adminUser.PopulateDefaults()
	adminUser.Privpwd = ygot.String("this-is-a-secret")
	return adminUser
}

func createShutdownItems() *nxos.Cisco_NX_OSDevice_System_SnmpItems_ServershutdownItems {
	shutdown := &nxos.Cisco_NX_OSDevice_System_SnmpItems_ServershutdownItems{}
	shutdown.PopulateDefaults()
	shutdown.SysShutdown = nxos.Cisco_NX_OSDevice_Snmp_Boolean_no
	return shutdown
}

func createTrapsItems() *nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems {
	traps := &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems{}
	traps.PopulateDefaults()
	traps.FcnsItems = nil
	traps.RscnItems = nil
	traps.ZoneItems = nil
	return traps
}

func Test_SNMP(t *testing.T) {
	s := &SNMP{
		Enable: true,
		Traps:  []string{"license notify-license-expiry"},
	}

	got, err := s.ToYGOT(t.Context(), mockedClient)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/snmp-items" {
		t.Errorf("expected key 'System/snmp-items' to be present")
	}

	v, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_SnmpItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_SnmpItems")
	}

	// check that we haven't modified the admin user password that we return in the mock
	adminUser := v.GetInstItems().GetLclUserItems().GetLocalUserList("admin")
	if adminUser == nil || !reflect.DeepEqual(adminUser, createAdminUser()) {
		t.Errorf("admin user has not been copied correctly")
	}

	// check that the shutdown items have been copied correctly from the mock
	shutdown := v.GetServershutdownItems()
	if shutdown == nil || !reflect.DeepEqual(shutdown, v.GetServershutdownItems()) {
		t.Errorf("shutdown items have not been copied correctly")
	}

	if v.InstItems.TrapsItems.LicenseItems.NotifylicenseexpiryItems.Trapstatus != nxos.Cisco_NX_OSDevice_Snmp_SnmpTrapSt_enable {
		t.Errorf("expected value for 'System/snmp-items/inst-items/traps-items/license-items/notifylicenseexpiry-items/trapstatus' to be enabled, got %v", v.InstItems.TrapsItems.LicenseItems.NotifylicenseexpiryItems.Trapstatus)
	}
}

func Test_SNMP_Err(t *testing.T) {
	s := &SNMP{
		Enable: true,
		Traps:  []string{"license invalid"},
	}

	if _, err := s.ToYGOT(t.Context(), mockedClient); err == nil {
		t.Errorf("expected error, got nil")
	}
}

func Test_SNMP_Reset(t *testing.T) {
	s := &SNMP{
		Enable: true,
	}

	updates, err := s.Reset(t.Context(), mockedClient)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(updates))
	}

	update, ok := updates[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}

	if update.XPath != "System/snmp-items" {
		t.Errorf("expected key 'System/snmp-items' to be present, got %s", update.XPath)
	}

	v, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_SnmpItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_SnmpItems")
	}

	// we need an empty SNMP structure with the admin user and shutdown items populated as in the mock.
	// The rest of the structure should be empty as the defaults
	emptySNMP, err := s.createAndPopulateSNMPItems()
	emptySNMP.PopulateDefaults()
	if err != nil {
		t.Errorf("unexpected error creating empty SNMP items: %v", err)
	}
	adminUser := emptySNMP.GetOrCreateInstItems().GetOrCreateLclUserItems().GetOrCreateLocalUserList("admin")
	*adminUser = *createAdminUser()

	traps := emptySNMP.GetOrCreateInstItems().GetOrCreateTrapsItems()
	*traps = *createTrapsItems()

	shutdown := emptySNMP.GetOrCreateServershutdownItems()
	*shutdown = *createShutdownItems()

	// check that the returned SNMP structure is equal to the empty SNMP structure with admin user and shutdown items populated
	if !reflect.DeepEqual(v, emptySNMP) {
		t.Errorf("expected value to be equal to empty SNMP structure, got %v", v)
	}
}
