// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_NXAPI(t *testing.T) {
	nxapi := &NXAPI{
		Enable: true,
	}

	got, err := nxapi.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}
	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/fm-items/nxapi-items" {
		t.Errorf("expected key 'System/fm-items/nxapi-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected value for 'System/fm-items/nxapi-items/adminSt' to be enabled")
	}
}

func Test_NXAPI_Trustpoint(t *testing.T) {
	nxapi := &NXAPI{
		Enable: true,
		Cert:   Trustpoint{ID: "mytrustpoint"},
	}

	got, err := nxapi.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 keys, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/fm-items/nxapi-items" {
		t.Errorf("expected key 'System/fm-items/nxapi-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected value for 'System/fm-items/nxapi-items/adminSt' to be enabled")
	}

	update, ok = got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/nxapi-items" {
		t.Errorf("expected key 'System/nxapi-items' to be present")
	}

	g, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_NxapiItems")
	}

	want := &nxos.Cisco_NX_OSDevice_System_NxapiItems{
		Trustpoint: ygot.String("mytrustpoint"),
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("unexpected value for 'System/nxapi-items': got=%+v, want=%+v", g, want)
	}
}

func Test_NXAPI_Cert(t *testing.T) {
	nxapi := &NXAPI{
		Enable: true,
		Cert: Certificate{
			CertFile:   "cert.pem",
			KeyFile:    "key.pem",
			Passphrase: "passphrase",
		},
	}

	got, err := nxapi.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 keys, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/fm-items/nxapi-items" {
		t.Errorf("expected key 'System/fm-items/nxapi-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected value for 'System/fm-items/nxapi-items/adminSt' to be enabled")
	}

	update, ok = got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/nxapi-items" {
		t.Errorf("expected key 'System/nxapi-items' to be present")
	}

	g, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_NxapiItems")
	}

	want := &nxos.Cisco_NX_OSDevice_System_NxapiItems{
		CertFile:          ygot.String("cert.pem"),
		CertEnable:        ygot.Bool(true),
		KeyFile:           ygot.String("key.pem"),
		EncrKeyPassphrase: ygot.String("passphrase"),
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("unexpected value for 'System/nxapi-items': got=%+v, want=%+v", g, want)
	}
}

func Test_NXAPI_Disabled(t *testing.T) {
	nxapi := &NXAPI{Enable: false}

	got, err := nxapi.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/fm-items/nxapi-items" {
		t.Errorf("expected key 'System/fm-items/nxapi-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled {
		t.Errorf("expected value for 'System/fm-items/nxapi-items/adminSt' to be disabled")
	}
}
