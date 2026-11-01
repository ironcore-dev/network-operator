// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"reflect"
	"testing"
)

func TestSwitchportAddAllowedVLAN(t *testing.T) {
	tests := []struct {
		name       string
		switchport Switchport
		vlan       int32
		want       []IndexRange
		changed    bool
		wantErr    bool
	}{
		{
			name:       "takes ownership of nil list",
			switchport: Switchport{Mode: SwitchportModeTrunk},
			vlan:       100,
			want:       []IndexRange{{Start: 100, End: 100}},
			changed:    true,
		},
		{
			name: "keeps containing range",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 90, End: 110}},
			},
			vlan: 100,
			want: []IndexRange{{Start: 90, End: 110}},
		},
		{
			name: "adds sorted singleton",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 200, End: 210}},
			},
			vlan:    100,
			want:    []IndexRange{{Start: 100, End: 100}, {Start: 200, End: 210}},
			changed: true,
		},
		{
			name:       "rejects access mode",
			switchport: Switchport{Mode: SwitchportModeAccess},
			vlan:       100,
			wantErr:    true,
		},
		{
			name: "rejects unmanaged mode",
			switchport: Switchport{
				Mode:             SwitchportModeTrunk,
				AllowedVlansMode: AllowedVlansModeUnmanaged,
			},
			vlan:    100,
			wantErr: true,
		},
		{
			name:       "rejects invalid VLAN",
			switchport: Switchport{Mode: SwitchportModeTrunk},
			vlan:       4095,
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			switchport := test.switchport
			changed, err := switchport.AddAllowedVLAN(test.vlan)
			if test.wantErr {
				if err == nil {
					t.Fatalf("AddAllowedVLAN(%d) returned no error", test.vlan)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddAllowedVLAN(%d): %v", test.vlan, err)
			}
			if changed != test.changed {
				t.Fatalf("AddAllowedVLAN(%d) changed = %t, want %t", test.vlan, changed, test.changed)
			}
			if !reflect.DeepEqual(switchport.AllowedVlans, test.want) {
				t.Fatalf("AddAllowedVLAN(%d) ranges = %#v, want %#v", test.vlan, switchport.AllowedVlans, test.want)
			}
		})
	}
}

func TestSwitchportRemoveAllowedVLAN(t *testing.T) {
	tests := []struct {
		name       string
		switchport Switchport
		vlan       int32
		want       []IndexRange
		changed    bool
		wantErr    bool
	}{
		{
			name: "keeps absent VLAN",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 10, End: 20}},
			},
			vlan: 100,
			want: []IndexRange{{Start: 10, End: 20}},
		},
		{
			name: "removes final VLAN",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 100, End: 100}},
			},
			vlan:    100,
			changed: true,
		},
		{
			name: "shrinks range start",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 100, End: 110}},
			},
			vlan:    100,
			want:    []IndexRange{{Start: 101, End: 110}},
			changed: true,
		},
		{
			name: "shrinks range end",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 100, End: 110}},
			},
			vlan:    110,
			want:    []IndexRange{{Start: 100, End: 109}},
			changed: true,
		},
		{
			name: "splits range",
			switchport: Switchport{
				Mode:         SwitchportModeTrunk,
				AllowedVlans: []IndexRange{{Start: 100, End: 110}},
			},
			vlan:    105,
			want:    []IndexRange{{Start: 100, End: 104}, {Start: 106, End: 110}},
			changed: true,
		},
		{
			name:       "rejects access mode",
			switchport: Switchport{Mode: SwitchportModeAccess},
			vlan:       100,
			wantErr:    true,
		},
		{
			name: "rejects unmanaged mode",
			switchport: Switchport{
				Mode:             SwitchportModeTrunk,
				AllowedVlansMode: AllowedVlansModeUnmanaged,
			},
			vlan:    100,
			wantErr: true,
		},
		{
			name:       "rejects invalid VLAN",
			switchport: Switchport{Mode: SwitchportModeTrunk},
			vlan:       0,
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			switchport := test.switchport
			changed, err := switchport.RemoveAllowedVLAN(test.vlan)
			if test.wantErr {
				if err == nil {
					t.Fatalf("RemoveAllowedVLAN(%d) returned no error", test.vlan)
				}
				return
			}
			if err != nil {
				t.Fatalf("RemoveAllowedVLAN(%d): %v", test.vlan, err)
			}
			if changed != test.changed {
				t.Fatalf("RemoveAllowedVLAN(%d) changed = %t, want %t", test.vlan, changed, test.changed)
			}
			if !reflect.DeepEqual(switchport.AllowedVlans, test.want) {
				t.Fatalf("RemoveAllowedVLAN(%d) ranges = %#v, want %#v", test.vlan, switchport.AllowedVlans, test.want)
			}
		})
	}
}
