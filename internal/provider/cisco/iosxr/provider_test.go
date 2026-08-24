// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tidwall/gjson"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

type TestCase struct {
	name string
	val  gnmiext.DataElement
}

var tests []TestCase

func Register(name string, val gnmiext.DataElement) {
	tests = append(tests, TestCase{
		name: name,
		val:  val,
	})
}

func removeRootElement(xpath string) string {
	parts := strings.Split(xpath, ":")
	if len(parts) == 1 {
		return xpath
	}
	return parts[1]
}

func Test_Payload(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(test.val)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			file := "testdata/" + test.name + ".json"
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", file, err)
			}

			var buf bytes.Buffer
			if err := json.Compact(&buf, data); err != nil {
				t.Errorf("json.Compact() error = %v", err)
				return
			}

			xpath := removeRootElement(test.val.XPath())
			path, err := gnmiext.StringToStructuredPath(xpath)
			if err != nil {
				t.Errorf("StringToStructuredPath(%q) error = %v", xpath, err)
				return
			}

			var sb strings.Builder
			for _, elem := range path.GetElem() {
				if elem.GetName() == "" {
					continue
				}
				if sb.Len() > 0 {
					sb.WriteByte('|')
				}
				sb.WriteString(elem.GetName())
			}

			res := gjson.GetBytes(buf.Bytes(), sb.String())
			if want := []byte(res.Raw); !jsonEqual(want, b) {
				t.Errorf("payload mismatch:\nwant: %s\ngot:  %s", want, b)
			}
		})
	}
}

// sortSlices recursively sorts all slices in a JSON-unmarshaled structure
// to ensure deterministic comparison regardless of map iteration order.
func sortSlices(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = sortSlices(item)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sortSlices(item)
		}
		slices.SortFunc(result, func(i, j any) int {
			a, err := json.Marshal(i)
			if err != nil {
				return 0
			}
			b, err := json.Marshal(j)
			if err != nil {
				return 0
			}
			return bytes.Compare(a, b)
		})
		return result
	default:
		return v
	}
}

var jsonNormalizer = cmpopts.AcyclicTransformer("sortSlices", sortSlices)

// jsonEqual compares two JSON byte slices for semantic equality,
// treating arrays as unordered sets (appropriate for YANG list nodes).
func jsonEqual(a, b []byte) bool {
	var v1, v2 any
	if err := json.Unmarshal(a, &v1); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &v2); err != nil {
		return false
	}
	return cmp.Equal(v1, v2, jsonNormalizer)
}

func Test_EnsureInterface(t *testing.T) {
	m := &gnmiext.ClientMock{
		UpdateFunc: func(ctx context.Context, el ...gnmiext.DataElement) error {
			return nil
		},
	}
	p := &Provider{client: m}

	ctx := t.Context()

	name := "TwentyFiveGigE0/0/0/14"
	var prefix netip.Prefix

	prefix, err := netip.ParsePrefix("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to parse prefix: %v", err)
	}

	ipv4 := v1alpha1.InterfaceIPv4{
		Addresses: []v1alpha1.IPPrefix{
			{
				Prefix: prefix,
			},
		},
	}

	req := &provider.EnsureInterfaceRequest{
		Interface: &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				Name:        name,
				IPv4:        &ipv4,
				Description: "i572056-test-2",
				AdminState:  "UP",
				Type:        "Physical",
				MTU:         9600,
			},
		},
	}

	err = p.EnsureInterface(ctx, req)
	if err != nil {
		t.Fatalf("EnsureInterface() error = %v", err)
	}
}

func Test_GetState(t *testing.T) {
	m := &gnmiext.ClientMock{
		GetStateFunc: func(ctx context.Context, states ...gnmiext.DataElement) error {
			states[0].(*PhysIfState).State = "im-state-up"
			return nil
		},
	}

	p := &Provider{
		client: m,
		conn:   nil,
	}

	ctx := t.Context()
	name := "TwentyFiveGigE0/0/0/14"

	req := &provider.InterfaceRequest{
		Interface: &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				Name: name,
			},
		},
	}

	status, err := p.GetInterfaceStatus(ctx, req)
	if err != nil {
		t.Fatalf("EnsureInterface() error = %v", err)
	}

	if !status.OperStatus {
		t.Fatalf("GetInterfaceStatus() expected OperStatus=true, got false")
	}
}

func Test_NewMTU(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		mtu           int32
		wantMTU       int32
		wantErr       bool
	}{
		{
			name:          "empty MTU should return default link MTU",
			interfaceName: "TwentyFiveGigE0/0/0/14",
			mtu:           0,
			wantMTU:       DefaultLinkMTU,
			wantErr:       false,
		},
		{
			name:          "jumbo MTU value of 9000",
			interfaceName: "TwentyFiveGigE0/0/0/14",
			mtu:           9000,
			wantMTU:       9000,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMTU(tt.interfaceName, tt.mtu)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMTU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got.MTU) == 0 {
					t.Errorf("NewMTU() returned empty MTU slice")
					return
				}
				if got.MTU[0].MTU != tt.wantMTU {
					t.Errorf("NewMTU() MTU = %v, want %v", got.MTU[0].MTU, tt.wantMTU)
				}
				if got.MTU[0].Owner != "TwentyFiveGigE" {
					t.Errorf("NewMTU() Owner = %v, want TwentyFiveGigE", got.MTU[0].Owner)
				}
			}
		})
	}
}
