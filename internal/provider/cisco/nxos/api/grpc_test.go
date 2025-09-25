// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/testutils"
)

func Test_GRPC_ToYGOT(t *testing.T) {
	tests := []struct {
		name     string
		grpc     *GRPC
		expected []gnmiext.Update
	}{
		{
			name: "disabled",
			grpc: &GRPC{Enable: false},
			expected: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/grpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
					},
				},
			},
		},
		{
			name: "enabled with defaults",
			grpc: &GRPC{
				Enable:     true,
				Vrf:        "CC-MGMT",
				Trustpoint: "mytrustpoint",
			},
			expected: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/grpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.EditingUpdate{
					XPath: "System/grpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_GrpcItems{
						Cert:           ygot.String("mytrustpoint"),
						Port:           ygot.Uint32(50051),
						UseVrf:         ygot.String("CC-MGMT"),
						CertClientRoot: nil,
						GnmiItems: &nxos.Cisco_NX_OSDevice_System_GrpcItems_GnmiItems{
							MaxCalls:          ygot.Uint16(8),
							KeepAliveTimeout:  ygot.Uint32(600),
							MinSampleInterval: ygot.Uint32(10),
						},
					},
				},
			},
		},
		{
			name: "enabled with custom values",
			grpc: &GRPC{
				Enable:         true,
				Port:           9000,
				Vrf:            "production",
				Trustpoint:     "custom-trustpoint",
				CertClientRoot: "root-cert",
				GNMI: &GNMI{
					MaxConcurrentCall: 12,
					KeepAliveTimeout:  900,
					MinSampleInterval: 5,
				},
			},
			expected: []gnmiext.Update{
				gnmiext.EditingUpdate{
					XPath: "System/fm-items/grpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems{
						AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
					},
				},
				gnmiext.EditingUpdate{
					XPath: "System/grpc-items",
					Value: &nxos.Cisco_NX_OSDevice_System_GrpcItems{
						Cert:           ygot.String("custom-trustpoint"),
						Port:           ygot.Uint32(9000),
						UseVrf:         ygot.String("production"),
						CertClientRoot: ygot.String("root-cert"),
						GnmiItems: &nxos.Cisco_NX_OSDevice_System_GrpcItems_GnmiItems{
							MaxCalls:          ygot.Uint16(12),
							KeepAliveTimeout:  ygot.Uint32(900),
							MinSampleInterval: ygot.Uint32(5),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.grpc.ToYGOT(t.Context(), &gnmiext.ClientMock{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			testutils.AssertEqual(t, got, tt.expected)
		})
	}
}

func Test_GRPC_Reset(t *testing.T) {
	grpc := &GRPC{} // Any configuration should be ignored for Reset

	got, err := grpc.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/grpc-items",
			Value: &nxos.Cisco_NX_OSDevice_System_GrpcItems{
				Port:   ygot.Uint32(50051),
				UseVrf: ygot.String("default"),
				GnmiItems: &nxos.Cisco_NX_OSDevice_System_GrpcItems_GnmiItems{
					MaxCalls:          ygot.Uint16(8),
					KeepAliveTimeout:  ygot.Uint32(600),
					MinSampleInterval: ygot.Uint32(10),
				},
			},
		},
	}

	testutils.AssertEqual(t, got, expected)
}
