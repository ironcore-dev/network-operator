// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package snmp

import (
	"context"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestVersionFrom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Version
	}{
		{
			name:  "v1",
			input: "v1",
			want:  V1,
		},
		{
			name:  "v2c",
			input: "v2c",
			want:  V2c,
		},
		{
			name:  "v3",
			input: "v3",
			want:  V3,
		},
		{
			name:  "invalid version defaults to v1",
			input: "invalid",
			want:  V1,
		},
		{
			name:  "empty string defaults to v1",
			input: "",
			want:  V1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := VersionFrom(test.input); got != test.want {
				t.Errorf("VersionFrom(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestMessageTypeFrom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  MessageType
	}{
		{
			name:  "traps",
			input: "traps",
			want:  Traps,
		},
		{
			name:  "informs",
			input: "informs",
			want:  Informs,
		},
		{
			name:  "invalid type defaults to traps",
			input: "invalid",
			want:  Traps,
		},
		{
			name:  "empty string defaults to traps",
			input: "",
			want:  Traps,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MessageTypeFrom(test.input); got != test.want {
				t.Errorf("MessageTypeFrom(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestSNMP_ToYGOT(t *testing.T) {
	tests := []struct {
		name            string
		snmp            *SNMP
		mockGet         func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error
		wantErr         bool
		wantUpdateCount int
		wantContact     string
		wantLocation    string
	}{
		{
			name: "basic configuration",
			snmp: &SNMP{
				Contact:  "admin@example.com",
				Location: "Data Center 1",
				SrcIf:    "mgmt0",
				IPv4ACL:  "SNMP-ACL",
				Hosts: []Host{
					{
						Address:   "192.168.1.100",
						Community: "public",
						Version:   V2c,
						Type:      Traps,
						Vrf:       "management",
					},
				},
				Communities: []Community{
					{
						Name:    "public",
						Group:   "network-admin",
						IPv4ACL: "SNMP-READ",
					},
				},
				Traps: []string{},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
					res.LocalUserList["admin"] = &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList{}
				}
				return nil
			},
			wantErr:         false,
			wantUpdateCount: 7,
			wantContact:     "admin@example.com",
			wantLocation:    "Data Center 1",
		},
		{
			name: "duplicate host error",
			snmp: &SNMP{
				Hosts: []Host{
					{
						Address:   "192.168.1.100",
						Community: "public",
						Version:   V1,
						Type:      Traps,
					},
					{
						Address:   "192.168.1.100", // Duplicate address
						Community: "private",
						Version:   V2c,
						Type:      Informs,
					},
				},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "duplicate community error",
			snmp: &SNMP{
				Communities: []Community{
					{
						Name:  "public",
						Group: "network-admin",
					},
					{
						Name:  "public", // Duplicate name
						Group: "network-operator",
					},
				},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "invalid trap configuration",
			snmp: &SNMP{
				Traps: []string{"invalid-trap-name"},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "v3 host configuration",
			snmp: &SNMP{
				Hosts: []Host{
					{
						Address:   "192.168.1.200",
						Community: "snmpv3user",
						Version:   V3,
						Type:      Informs,
						Vrf:       "default",
					},
				},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr:         false,
			wantUpdateCount: 6,
		},
		{
			name: "empty configuration",
			snmp: &SNMP{},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr:         false,
			wantUpdateCount: 6,
		},
		{
			name: "empty strings use unset marker",
			snmp: &SNMP{
				Contact:  "", // Empty contact should use DME_UNSET_PROPERTY_MARKER
				Location: "", // Empty location should use DME_UNSET_PROPERTY_MARKER
				SrcIf:    "", // Empty source interface should use DME_UNSET_PROPERTY_MARKER
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr:         false,
			wantUpdateCount: 6,
			wantContact:     "DME_UNSET_PROPERTY_MARKER",
			wantLocation:    "DME_UNSET_PROPERTY_MARKER",
		},
		{
			name: "version and message type mappings",
			snmp: &SNMP{
				Hosts: []Host{
					{
						Address:   "192.168.1.10",
						Community: "v1-community",
						Version:   V1,
						Type:      Traps,
					},
					{
						Address:   "192.168.1.20",
						Community: "v2c-community",
						Version:   V2c,
						Type:      Informs,
					},
				},
			},
			mockGet: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
				if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
					res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				}
				return nil
			},
			wantErr:         false,
			wantUpdateCount: 6,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &gnmiext.ClientMock{
				GetFunc: test.mockGet,
			}

			updates, err := test.snmp.ToYGOT(context.Background(), mockClient)
			if test.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if test.wantUpdateCount > 0 {
				if len(updates) != test.wantUpdateCount {
					t.Errorf("expected %d updates, got %d", test.wantUpdateCount, len(updates))
				}
			}

			if test.wantContact != "" || test.wantLocation != "" {
				sysinfoUpdate, ok := updates[0].(gnmiext.EditingUpdate)
				if !ok {
					t.Errorf("expected EditingUpdate for sysinfo")
				}
				xpath := "System/snmp-items/inst-items/sysinfo-items"
				if sysinfoUpdate.XPath != xpath {
					t.Errorf("expected XPath %s, got %s", xpath, sysinfoUpdate.XPath)
				}
				sysinfo, ok := sysinfoUpdate.Value.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_SysinfoItems)
				if !ok {
					t.Errorf("expected sysinfo value type")
				}
				if test.wantContact != "" && *sysinfo.SysContact != test.wantContact {
					t.Errorf("expected contact %s, got %s", test.wantContact, *sysinfo.SysContact)
				}
				if test.wantLocation != "" && *sysinfo.SysLocation != test.wantLocation {
					t.Errorf("expected location %s, got %s", test.wantLocation, *sysinfo.SysLocation)
				}
			}
		})
	}
}

func TestSNMP_Reset(t *testing.T) {
	mockClient := &gnmiext.ClientMock{
		GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
			if res, ok := dest.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems); ok {
				res.LocalUserList = make(map[string]*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList)
				res.LocalUserList["admin"] = &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList{}
				res.LocalUserList["operator"] = &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList{}
			}
			return nil
		},
	}

	updates, err := (&SNMP{}).Reset(context.Background(), mockClient)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if len(updates) != 8 {
		t.Errorf("expected 8 updates, got %d", len(updates))
		return
	}

	del, ok := updates[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 0: expected DeletingUpdate, got %T", updates[0])
	}
	xpath := "System/snmp-items/inst-items/sysinfo-items"
	if del.XPath != xpath {
		t.Errorf("update 0: expected XPath %s, got %s", xpath, del.XPath)
	}

	del, ok = updates[1].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 1: expected DeletingUpdate, got %T", updates[1])
	}
	xpath = "System/snmp-items/inst-items/globals-items/srcInterfaceTraps-items"
	if del.XPath != xpath {
		t.Errorf("update 1: expected XPath %s, got %s", xpath, del.XPath)
	}

	del, ok = updates[2].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 2: expected DeletingUpdate, got %T", updates[2])
	}
	xpath = "System/snmp-items/inst-items/globals-items/srcInterfaceInforms-items"
	if del.XPath != xpath {
		t.Errorf("update 2: expected XPath %s, got %s", xpath, del.XPath)
	}

	del, ok = updates[3].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 3: expected DeletingUpdate, got %T", updates[3])
	}
	xpath = "System/snmp-items/inst-items/host-items"
	if del.XPath != xpath {
		t.Errorf("update 3: expected XPath %s, got %s", xpath, del.XPath)
	}

	del, ok = updates[4].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 4: expected DeletingUpdate, got %T", updates[4])
	}
	xpath = "System/snmp-items/inst-items/community-items"
	if del.XPath != xpath {
		t.Errorf("update 4: expected XPath %s, got %s", xpath, del.XPath)
	}

	replace, ok := updates[5].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("update 5: expected ReplacingUpdate, got %T", updates[5])
	}
	xpath = "System/snmp-items/inst-items/traps-items"
	if replace.XPath != xpath {
		t.Errorf("update 5: expected XPath %s, got %s", xpath, replace.XPath)
	}
	traps, ok := replace.Value.(*nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems)
	if !ok {
		t.Errorf("update 5: expected TrapsItems value, got %T", replace.Value)
	}
	if *traps != (nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems{}) {
		t.Errorf("update 5: expected empty TrapsItems, got %+v", traps)
	}

	del, ok = updates[6].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 6: expected DeletingUpdate, got %T", updates[6])
	}
	xpath = "System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=admin]/ipv4AclName"
	if del.XPath != xpath {
		t.Errorf("update 6: expected XPath %s, got %s", xpath, del.XPath)
	}

	del, ok = updates[7].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("update 7: expected DeletingUpdate, got %T", updates[7])
	}
	xpath = "System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=operator]/ipv4AclName"
	if del.XPath != xpath {
		t.Errorf("update 7: expected XPath %s, got %s", xpath, del.XPath)
	}
}
