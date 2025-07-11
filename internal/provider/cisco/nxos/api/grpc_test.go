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

func Test_GRPC(t *testing.T) {
	grpc := &GRPC{
		Enable:     true,
		Vrf:        "CC-MGMT",
		Trustpoint: "mytrustpoint",
	}

	got, err := grpc.ToYGOT(&gnmiext.ClientMock{})
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

	if update.XPath != "System/fm-items/grpc-items" {
		t.Errorf("expected key 'System/fm-items/grpc-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled {
		t.Errorf("expected value for 'System/fm-items/grpc-items/adminSt' to be enabled")
	}

	update, ok = got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/grpc-items" {
		t.Errorf("expected key 'System/grpc-items' to be present")
	}

	g, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_GrpcItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems")
	}

	want := &nxos.Cisco_NX_OSDevice_System_GrpcItems{
		Cert: ygot.String("mytrustpoint"),
		GnmiItems: &nxos.Cisco_NX_OSDevice_System_GrpcItems_GnmiItems{
			MinSampleInterval: ygot.Uint32(10),
			KeepAliveTimeout:  ygot.Uint32(600),
			MaxCalls:          ygot.Uint16(8),
		},
		Port:   ygot.Uint32(50051),
		UseVrf: ygot.String("CC-MGMT"),
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("unexpected value for 'System/grpc-items': got=%+v, want=%+v", g, want)
	}
}

func Test_GRPC_Disabled(t *testing.T) {
	grpc := &GRPC{Enable: false}

	got, err := grpc.ToYGOT(&gnmiext.ClientMock{})
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

	if update.XPath != "System/fm-items/grpc-items" {
		t.Errorf("expected key 'System/fm-items/grpc-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems")
	}
	if i.AdminSt != nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled {
		t.Errorf("expected value for 'System/fm-items/grpc-items/adminSt' to be disabled")
	}
}
