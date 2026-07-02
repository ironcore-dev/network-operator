// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"reflect"
	"testing"
)

func init() {
	rd, _ := NewRouteDistinguisher("1000:100") //nolint:errcheck
	peer := BGPPeer{
		RouterID: "11111.1111",
		AF: ActivatedAddressFamilies{
			AF: []ActivatedAddressFamily{
				{
					AFName: string(AfNameIpv4Unicast),
					Redistribute: Redistribute{
						Static: Static{},
					},
				},
			},
		},
		Name: "testvrf",
		RD:   rd,
		Neighbors: NeighborList{
			Neighbors: []Neighbor{
				{
					NeighborAddress: "127.0.0.1",
					AF: NeighborAddressFamilies{
						AF: []NeighborAddressFamily{
							{
								AfName: "ipv4-unicast",
								MaximumPrefix: MaximumPrefix{
									PrefixLimit: 100,
									Restart:     15,
									Threshold:   80,
								},
								RoutePolicy: PeeringRPL{
									In:  "RPL_testvrf",
									Out: "RPL_testvrf",
								},
								SendCommunityEbgp:         SendCommunityEbgp{},
								SendCommunityGShutEbgp:    SendCommunityGShutEbgp{},
								SendExtendedCommunityEbgp: SendExtendedCommunityEbgp{},
								SoftReconfiguration: SoftReconfiguration{
									Inbound: Inbound{
										Always: true,
									},
								},
							},
						},
					},
					LocalAS: LocalAS{
						AS: AS{
							ASNumber: 65000,
							NoPrepend: PrependAS{
								ReplaceAS: ReplaceAS{},
							},
						},
					},
					RemoteAS: 65001,
					SessionConfig: SessionConfig{
						SessionGroup: "EBGP-CUSTOMER-DEFAULTS",
					},
				},
			},
		},
	}
	Register("bgppeer", &peer)
}

func TestNewRouteDistinguisher(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected RouteDistinguisher
		wantErr  bool
	}{
		{
			name:  "Type 0 (2 Byte) ASN",
			input: "1000:100",
			expected: RouteDistinguisher{
				TwoByteAS: TwoByteAS{
					ASNumber: 1000,
					Index:    100,
				},
			},
			wantErr: false,
		},
		{
			name:  "Type 2 (4 Byte) ASN",
			input: "4294967295:65535",
			expected: RouteDistinguisher{
				FourByteAS: FourByteAS{
					ASNumber: 4294967295,
					Index:    65535,
				},
			},
			wantErr: false,
		},
		{
			name:  "IPv4 address with max index",
			input: "10.0.0.1:65535",
			expected: RouteDistinguisher{
				IPAddress: IPAddressAS{
					Address: "10.0.0.1",
					Index:   65535,
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid format - no colon",
			input:    "1000",
			expected: RouteDistinguisher{},
			wantErr:  true,
		},
		{
			name:     "invalid format - too many parts",
			input:    "1000:100:200",
			expected: RouteDistinguisher{},
			wantErr:  true,
		},
		{
			name:     "invalid format - empty string",
			input:    "",
			expected: RouteDistinguisher{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd, err := NewRouteDistinguisher(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewRouteDistinguisher(%s) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("NewRouteDistinguisher(%s) unexpected error: %v", tt.input, err)
				}
				if !reflect.DeepEqual(rd, tt.expected) {
					t.Errorf("NewRouteDistinguisher(%s) = %+v, want %+v", tt.input, rd, tt.expected)
				}
			}
		})
	}
}
