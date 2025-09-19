// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package vpc

import (
	"context"
	"errors"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
)

func Test_NewVPC(t *testing.T) {
	tests := []struct {
		name        string
		domainID    int
		options     []Option
		shouldError bool
	}{
		// empty options
		{
			name:        "valid: domain ID in range",
			domainID:    10,
			options:     nil,
			shouldError: false,
		},
		{
			name:        "invalid: domain ID too low",
			domainID:    0,
			options:     nil,
			shouldError: true,
		},
		{
			name:        "invalid: domain ID too high",
			domainID:    1001,
			options:     nil,
			shouldError: true,
		},
		{
			name:     "valid: WithPeerLink minimal config",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "192.168.1.1",
				}),
			},
			shouldError: false,
		},
		// with peer-link option
		{
			name:     "invalid: WithPeerLink with invalid port-channel name",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel: "eth1",
				}),
			},
			shouldError: true,
		},
		{
			name:     "invalid: WithPeerLink without port-channel name",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					KeepAliveDstIP: "127.0.0.1",
				}),
			},
			shouldError: true,
		},
		{
			name:     "invalid: WithPeerLink without destination IP",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel: "po10",
				}),
			},
			shouldError: true,
		},
		{
			name:     "invalid: WithPeerLink with invalid string as destination IP",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "260.260.260.260",
				}),
			},
			shouldError: true,
		},
		{
			name:     "valid: WithPeerLink with valid destination IPv4",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "127.0.0.1",
				}),
			},
			shouldError: false,
		},
		{
			name:     "valid: WithPeerLink with valid destination IPv6",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "2001:db8::1",
				}),
			},
			shouldError: false,
		},
		{
			name:     "invalid: WithPeerLink empty VRF string",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "192.168.1.1",
					KeepAliveVRF:   func() *string { s := ""; return &s }(),
				}),
			},
			shouldError: true,
		},
		{
			name:     "invalid: WithPeerLink mismatched IP versions",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "192.168.1.1",
					KeepAliveSrcIP: func() *string { s := "2001:db8::1"; return &s }(),
				}),
			},
			shouldError: true,
		},
		// with members option
		{
			name:        "invalid: WithMembers with empty list",
			domainID:    10,
			options:     []Option{WithMembers([]Member{})},
			shouldError: true,
		},
		{
			name:     "invalid: WithMembers with invalid port-channel name",
			domainID: 10,
			options: []Option{WithMembers([]Member{
				{PortChannel: "eth1", VPCID: 1},
			})},
			shouldError: true,
		},
		{
			name:     "invalid: WithMembers with vPC number too low",
			domainID: 10,
			options: []Option{WithMembers([]Member{
				{PortChannel: "po1", VPCID: 0},
			})},
			shouldError: true,
		},
		{
			name:     "invalid: WithMembers with vPC number too high",
			domainID: 10,
			options: []Option{WithMembers([]Member{
				{PortChannel: "po1", VPCID: 4097},
			})},
			shouldError: true,
		},
		{
			name:     "valid: WithMembers with multiple members",
			domainID: 10,
			options: []Option{
				WithMembers([]Member{
					{PortChannel: "po1", VPCID: 1},
					{PortChannel: "po2", VPCID: 2},
				}),
			},
			shouldError: false,
		},
		{
			name:     "invalid: WithMembers with multiple members and duplicate vPC IDs",
			domainID: 10,
			options: []Option{
				WithMembers([]Member{
					{PortChannel: "po1", VPCID: 1},
					{PortChannel: "po2", VPCID: 1},
				}),
			},
			shouldError: true,
		},
		{
			name:     "invalid: WithMembers with duplicate port-channel members",
			domainID: 10,
			options: []Option{
				WithMembers([]Member{
					{PortChannel: "po1", VPCID: 1},
					{PortChannel: "po1", VPCID: 2},
				}),
			},
			shouldError: true,
		},
		// combined options
		{
			name:     "valid: WithPeerLink and WithMembers",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "127.0.0.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 1},
				}),
			},
			shouldError: false,
		},
		{
			name:     "invalid: WithPeerLink and WithMembers use same port-channel",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "127.0.0.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po1", VPCID: 1},
				}),
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVPC(tt.domainID, tt.options...)
			if tt.shouldError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func Test_VPC_ToYGOT(t *testing.T) {
	tests := []struct {
		name            string
		domainID        int
		options         []Option
		expectedUpdates []gnmiext.Update
		clientMock      *gnmiext.ClientMock
		expectErr       bool
	}{
		{
			name:     "valid: minimal VPC",
			domainID: 10,
			options:  nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/vpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_VpcItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/vpc-items/inst-items/dom-items",
					Value: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems{
						Id:      ygot.Uint16(10),
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
					},
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(ctx context.Context, xpath string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					return nil
				},
			},
			expectErr: false,
		},
		{
			name:     "valid: full leaf configuration",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po10",
					KeepAliveDstIP: "192.168.1.1",
					KeepAliveSrcIP: func() *string { s := "192.168.1.2"; return &s }(),
					KeepAliveVRF:   func() *string { s := "a-random-vrf"; return &s }(),
				}),
				WithMembers([]Member{
					{PortChannel: "po1", VPCID: 1},
					{PortChannel: "po2", VPCID: 2},
					{PortChannel: "po3", VPCID: 3},
				}),
				EnablePeerGatewayFeature(),
				EnablePeerSwitchFeature(),
			},
			expectedUpdates: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/vpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_VpcItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.ReplacingUpdate{
					XPath: "System/vpc-items/inst-items/dom-items",
					Value: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems{
						Id:      ygot.Uint16(10),
						AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
						IfItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_IfItems{
							IfList: map[uint16]*nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_IfItems_IfList{
								1: {
									Id: ygot.Uint16(1),
									RsvpcConfItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_IfItems_IfList_RsvpcConfItems{
										TDn: ygot.String("/System/intf-items/aggr-items/AggrIf-list[id='po1']"),
									},
								},
								2: {
									Id: ygot.Uint16(2),
									RsvpcConfItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_IfItems_IfList_RsvpcConfItems{
										TDn: ygot.String("/System/intf-items/aggr-items/AggrIf-list[id='po2']"),
									},
								},
								3: {
									Id: ygot.Uint16(3),
									RsvpcConfItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_IfItems_IfList_RsvpcConfItems{
										TDn: ygot.String("/System/intf-items/aggr-items/AggrIf-list[id='po3']"),
									},
								},
							},
						},
						KeepaliveItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_KeepaliveItems{
							PeerlinkItems: &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems_KeepaliveItems_PeerlinkItems{
								Id: ygot.String("po10"),
							},
							DestIp: ygot.String("192.168.1.1"),
							SrcIp:  ygot.String("192.168.1.2"),
							Vrf:    ygot.String("a-random-vrf"),
						},
						PeerGw:     nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
						PeerSwitch: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
					},
				},
			},
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2
					destVal.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
					return nil
				},
				ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
					return true, nil
				},
			},
			expectErr: false,
		},

		{
			name:     "invalid: peer-link port-channel does not exist",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "192.168.1.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 100},
				}),
			},
			expectedUpdates: nil,
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					return errors.New("not found")
				},
			},
			expectErr: true,
		},
		{
			name:     "invalid: peer-link is not an L2 trunk (L3)",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "192.168.1.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 100},
				}),
			},
			expectedUpdates: nil,
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer3
					return nil
				},
			},
			expectErr: true,
		},
		{
			name:     "invalid: peer-link is not an L2 trunk (L2 access)",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "192.168.1.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 100},
				}),
			},
			expectedUpdates: nil,
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2
					destVal.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_access
					return nil
				},
			},
			expectErr: true,
		},
		{
			name:     "invalid: member does not exist",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "192.168.1.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 100},
				}),
			},
			expectedUpdates: nil,
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2
					destVal.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
					return nil
				},
				ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
					return false, nil
				},
			},
			expectErr: true,
		},
		{
			name:     "invalid: error while checking if member exists",
			domainID: 10,
			options: []Option{
				WithPeerLink(PeerLinkConfig{
					PortChannel:    "po1",
					KeepAliveDstIP: "192.168.1.1",
				}),
				WithMembers([]Member{
					{PortChannel: "po2", VPCID: 100},
				}),
			},
			expectedUpdates: nil,
			clientMock: &gnmiext.ClientMock{
				GetFunc: func(_ context.Context, _ string, dest ygot.GoStruct, opts ...gnmiext.GetOption) error {
					destVal, ok := dest.(*nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList)
					if !ok {
						return errors.New("unexpected type in GetFunc mock")
					}
					destVal.Layer = nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2
					destVal.Mode = nxos.Cisco_NX_OSDevice_L1_Mode_trunk
					return nil
				},
				ExistsFunc: func(_ context.Context, xpath string) (bool, error) {
					return false, errors.New("connection error")
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVPC(tt.domainID, tt.options...)
			if err != nil {
				if !tt.expectErr {
					t.Fatalf("unexpected error during NewVPC: %v", err)
				}
				return
			}
			updates, err := v.ToYGOT(t.Context(), tt.clientMock)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}

func Test_VPC_Reset(t *testing.T) {
	tests := []struct {
		name            string
		domainID        int
		options         []Option
		expectedUpdates []gnmiext.Update
	}{
		{
			name:     "valid: reset minimal VPC",
			domainID: 10,
			options:  nil,
			expectedUpdates: []gnmiext.Update{
				gnmiext.DeletingUpdate{
					XPath: "System/vpc-items/inst-items/dom-items",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVPC(tt.domainID, tt.options...)
			if err != nil {
				t.Fatalf("unexpected error during NewVPC: %v", err)
			}
			updates, err := v.Reset(t.Context(), nil)
			if err != nil {
				t.Fatalf("unexpected error during Reset: %v", err)
			}
			testutils.AssertEqual(t, updates, tt.expectedUpdates)
		})
	}
}
