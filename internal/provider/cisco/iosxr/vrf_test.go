// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFourByteRT(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedASN   uint32
		expectedIndex uint32
		expectedErr   string
	}{
		{
			name:          "valid route target",
			input:         "4268359684:1101",
			expectedASN:   4268359684,
			expectedIndex: 1101,
		},
		{
			name:        "invalid route target without colon",
			input:       "4268359684",
			expectedErr: "invalid route target format",
		},
		{
			name:        "invalid route target with invalid asn",
			input:       "invalid:1101",
			expectedErr: "invalid ASN",
		},
		{
			name:        "invalid route target with invalid index",
			input:       "4268359684:invalid",
			expectedErr: "invalid index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := NewFourByteRT(tt.input)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedASN, rt.ASNumber)
			assert.Equal(t, tt.expectedIndex, rt.Index)
		})
	}
}

func init() {
	rt := RouteTarget{
		FourByteAS: FourByteASRouteTargetList{
			Targets: FourByteASRouteTargetWrapper{
				Targets: []FourByteASRouteTarget{
					{ASNumber: 4268359684, Index: 1101, Stitching: Disable},
				},
			},
		},
	}

	Register("vrf", &VRF{
		Name:        "vrf-name",
		Description: "vrf-description",
		AddrFamily: AddressFamily{
			IPv4: UnicastFamily{
				Unicast: Unicast{Import: rt, Export: rt},
			},
			IPv6: UnicastFamily{
				Unicast: Unicast{Import: rt, Export: rt},
			},
		},
	})
}
