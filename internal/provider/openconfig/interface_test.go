// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestOpenConfigTrunkVlansJSON(t *testing.T) {
	trunkVlans := &TrunkVlans{
		InterfaceName: "ethernet-1/1",
		InterfaceType: v1alpha1.InterfaceTypePhysical,
		Vlans:         []any{uint16(10), "20..30"},
	}
	got, err := json.Marshal(trunkVlans)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(got) != `[10,"20..30"]` {
		t.Fatalf("json.Marshal() = %s, want %s", got, `[10,"20..30"]`)
	}

	var decoded TrunkVlans
	if err := json.Unmarshal([]byte(`[10,"20..30"]`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	want := []any{float64(10), "20..30"}
	if !reflect.DeepEqual(decoded.Vlans, want) {
		t.Fatalf("Unmarshaled Vlans = %#v, want %#v", decoded.Vlans, want)
	}
}

func TestOpenConfigTrunkVlansXPath(t *testing.T) {
	tests := []struct {
		name string
		in   *TrunkVlans
		want string
	}{
		{
			name: "physical",
			in:   &TrunkVlans{InterfaceName: "ethernet-1/1", InterfaceType: v1alpha1.InterfaceTypePhysical},
			want: "openconfig-interfaces:interfaces/interface[name=ethernet-1/1]/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/config/trunk-vlans",
		},
		{
			name: "aggregate",
			in:   &TrunkVlans{InterfaceName: "lag1", InterfaceType: v1alpha1.InterfaceTypeAggregate},
			want: "openconfig-interfaces:interfaces/interface[name=lag1]/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/config/trunk-vlans",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.in.XPath(); got != test.want {
				t.Fatalf("XPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInterfaceSetSwitchportOmitsTrunkVlans(t *testing.T) {
	i := &Interface{Name: "ethernet-1/1"}
	if err := i.SetSwitchport(&v1alpha1.Switchport{
		Mode:         v1alpha1.SwitchportModeTrunk,
		AllowedVlans: []v1alpha1.IndexRange{v1alpha1.MustParseIndexRange("10..10")},
	}, v1alpha1.InterfaceTypePhysical); err != nil {
		t.Fatalf("SetSwitchport() error = %v", err)
	}

	got := i.Ethernet.SwitchedVlan.Config
	if got.TrunkVlans != nil {
		t.Fatalf("TrunkVlans = %#v, want nil", got.TrunkVlans)
	}
	if got.InterfaceMode != SwitchportModeTrunk {
		t.Fatalf("InterfaceMode = %q, want %q", got.InterfaceMode, SwitchportModeTrunk)
	}
}
