// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testserver

import (
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestState_Set(t *testing.T) {
	tests := []struct {
		name    string
		initial []byte // starting buffer; defaults to `{}` when nil
		path    *gpb.Path
		raw     []byte
		want    map[string]string // gjson path -> expected string value ("" means must be absent)
	}{
		{
			name: "injects key into final keyed element",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
				},
			},
			raw:  []byte(`{"config":{"mtu":1500}}`),
			want: map[string]string{"interfaces.interface.0.name": "eth0"},
		},
		{
			name: "injects key into intermediate keyed element",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
					{Name: "config"},
				},
			},
			raw:  []byte(`{"mtu":1500}`),
			want: map[string]string{"interfaces.interface.0.name": "eth0"},
		},
		{
			name: "preserves existing key in raw payload",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
				},
			},
			raw:  []byte(`{"name":"eth0","config":{"mtu":1500}}`),
			want: map[string]string{"interfaces.interface.0.name": "eth0"},
		},
		{
			name: "handles multiple keys on one element",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "network-instances"},
					{Name: "network-instance", Key: map[string]string{"name": "default"}},
					{Name: "protocols"},
					{Name: "protocol", Key: map[string]string{"identifier": "BGP", "name": "bgp"}},
				},
			},
			raw: []byte(`{"config":{"enabled":true}}`),
			want: map[string]string{
				"network-instances.network-instance.0.protocols.protocol.0.identifier": "BGP",
				"network-instances.network-instance.0.protocols.protocol.0.name":       "bgp",
			},
		},
		{
			name: "materializes keys at every level of a deep path",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "network-instances"},
					{Name: "network-instance", Key: map[string]string{"name": "default"}},
					{Name: "protocols"},
					{Name: "protocol", Key: map[string]string{"identifier": "BGP", "name": "bgp"}},
					{Name: "bgp"},
					{Name: "neighbors"},
					{Name: "neighbor", Key: map[string]string{"neighbor-address": "10.0.0.1"}},
					{Name: "config"},
				},
			},
			raw: []byte(`{"peer-as":65001}`),
			want: map[string]string{
				"network-instances.network-instance.0.name":                                                           "default",
				"network-instances.network-instance.0.protocols.protocol.0.identifier":                                "BGP",
				"network-instances.network-instance.0.protocols.protocol.0.name":                                      "bgp",
				"network-instances.network-instance.0.protocols.protocol.0.bgp.neighbors.neighbor.0.neighbor-address": "10.0.0.1",
			},
		},
		{
			name:    "updates existing entry in place (no duplicates)",
			initial: []byte(`{"interfaces":{"interface":[{"name":"eth0","config":{"mtu":1500}}]}}`),
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
				},
			},
			raw: []byte(`{"config":{"mtu":9000}}`),
			want: map[string]string{
				"interfaces.interface.0.config.mtu": "9000",
				"interfaces.interface.#":            "1", // still a single entry
			},
		},
		{
			name:    "appends new entry for different key",
			initial: []byte(`{"interfaces":{"interface":[{"name":"eth0","config":{"mtu":1500}}]}}`),
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth1"}},
				},
			},
			raw: []byte(`{"config":{"mtu":1500}}`),
			want: map[string]string{
				"interfaces.interface.1.name": "eth1",
				"interfaces.interface.#":      "2",
			},
		},
		{
			name: "path without keys writes nested container",
			path: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "system"},
					{Name: "config"},
				},
			},
			raw:  []byte(`{"hostname":"router1"}`),
			want: map[string]string{"system.config.hostname": "router1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := tt.initial
			if buf == nil {
				buf = []byte(`{}`)
			}
			s := &State{Buf: buf}

			s.Set(tt.path, tt.raw)

			for path, want := range tt.want {
				got := gjson.GetBytes(s.Buf, path)
				if want == "" {
					assert.Falsef(t, got.Exists(), "expected %q to be absent, got %q in %s", path, got.String(), s.Buf)
					continue
				}
				require.Truef(t, got.Exists(), "expected %q to exist in %s", path, s.Buf)
				assert.Equalf(t, want, got.String(), "%q has wrong value in %s", path, s.Buf)
			}
		})
	}
}

func TestState_Set_ThenGet_RoundTrip(t *testing.T) {
	// Verifies key injection lets a later keyed Get find the entry.
	s := &State{Buf: []byte(`{}`)}

	path := &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "interfaces"},
			{Name: "interface", Key: map[string]string{"name": "eth0"}},
			{Name: "config"},
		},
	}
	s.Set(path, []byte(`{"mtu":1500}`))

	result := s.Get(path)
	require.NotEmpty(t, result, "Get should find the entry set with a keyed path")
	assert.Equal(t, "1500", gjson.GetBytes(result, "mtu").String())
}
