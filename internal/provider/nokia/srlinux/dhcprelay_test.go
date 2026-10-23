// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func TestEnsureDHCPRelayNetworkInstance(t *testing.T) {
	tests := []struct {
		name string
		vrf  *corev1alpha1.VRF
		want string
	}{
		{
			name: "no vrf leaves network-instance empty",
			vrf:  nil,
			want: "",
		},
		{
			name: "vrf sets network-instance to the device vrf name",
			vrf:  &corev1alpha1.VRF{Spec: corev1alpha1.VRFSpec{Name: "shared-services"}},
			want: "shared-services",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var captured *DHCPRelay
			p := &Provider{client: &gnmiext.ClientMock{
				UpdateFunc: func(_ context.Context, els ...gnmiext.DataElement) error {
					if r, ok := els[0].(*DHCPRelay); ok {
						captured = r
					}
					return nil
				},
			}}

			err := p.EnsureDHCPRelay(t.Context(), &provider.DHCPRelayRequest{
				DHCPRelay: &corev1alpha1.DHCPRelay{
					Spec: corev1alpha1.DHCPRelaySpec{Servers: []string{"192.0.2.1"}},
				},
				Interface: &corev1alpha1.Interface{
					Spec: corev1alpha1.InterfaceSpec{Name: "ethernet-1/1"},
				},
				VRF: test.vrf,
			})
			if err != nil {
				t.Fatalf("EnsureDHCPRelay() error = %v", err)
			}
			if captured == nil {
				t.Fatal("EnsureDHCPRelay() did not call Update")
			}
			if got := captured.NetworkInstance; got != test.want {
				t.Errorf("NetworkInstance = %q, want %q", got, test.want)
			}
		})
	}
}

// TestDHCPRelayJSONNetworkInstance guards the omitempty contract: an unset
// network-instance must be absent from the payload so periodic reconciles do
// not diff against the device default and trigger a spurious Set.
func TestDHCPRelayJSONNetworkInstance(t *testing.T) {
	tests := []struct {
		name    string
		relay   *DHCPRelay
		wantKey bool
	}{
		{
			name:    "empty network-instance is omitted",
			relay:   &DHCPRelay{AdminState: AdminStEnable, Server: []string{"192.0.2.1"}},
			wantKey: false,
		},
		{
			name:    "set network-instance is present",
			relay:   &DHCPRelay{AdminState: AdminStEnable, Server: []string{"192.0.2.1"}, NetworkInstance: "shared-services"},
			wantKey: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.relay)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if hasKey := strings.Contains(string(got), `"network-instance"`); hasKey != test.wantKey {
				t.Errorf("json.Marshal() = %s, network-instance present = %v, want %v", got, hasKey, test.wantKey)
			}
		})
	}
}

func TestDHCPRelayXPath(t *testing.T) {
	tests := []struct {
		name  string
		relay *DHCPRelay
		want  string
	}{
		{
			name:  "base subinterface",
			relay: &DHCPRelay{Interface: "ethernet-1/1"},
			want:  "srl_nokia-interfaces:interface[name=ethernet-1/1]/subinterface[index=0]/ipv4/srl_nokia-interfaces-ip-dhcp-relay:dhcp-relay",
		},
		{
			name:  "explicit subinterface index",
			relay: &DHCPRelay{Interface: "ethernet-1/1", SubinterfaceIndex: 100},
			want:  "srl_nokia-interfaces:interface[name=ethernet-1/1]/subinterface[index=100]/ipv4/srl_nokia-interfaces-ip-dhcp-relay:dhcp-relay",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.relay.XPath(); got != test.want {
				t.Errorf("XPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSplitSubinterface(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantIface string
		wantIndex int
		wantErr   bool
	}{
		{name: "no suffix defaults to zero", in: "ethernet-1/1", wantIface: "ethernet-1/1", wantIndex: 0},
		{name: "explicit subinterface", in: "ethernet-1/1.100", wantIface: "ethernet-1/1", wantIndex: 100},
		{name: "loopback", in: "lo0", wantIface: "lo0", wantIndex: 0},
		{name: "non-numeric suffix errors", in: "ethernet-1/1.abc", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iface, index, err := splitSubinterface(test.in)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("splitSubinterface(%q) error = %v", test.in, err)
			}
			if iface != test.wantIface || index != test.wantIndex {
				t.Errorf("splitSubinterface(%q) = (%q, %d), want (%q, %d)", test.in, iface, index, test.wantIface, test.wantIndex)
			}
		})
	}
}

func TestEnsureDHCPRelaySubinterface(t *testing.T) {
	var captured *DHCPRelay
	p := &Provider{client: &gnmiext.ClientMock{
		UpdateFunc: func(_ context.Context, els ...gnmiext.DataElement) error {
			if r, ok := els[0].(*DHCPRelay); ok {
				captured = r
			}
			return nil
		},
	}}

	err := p.EnsureDHCPRelay(t.Context(), &provider.DHCPRelayRequest{
		DHCPRelay: &corev1alpha1.DHCPRelay{
			Spec: corev1alpha1.DHCPRelaySpec{Servers: []string{"192.0.2.1"}},
		},
		Interface: &corev1alpha1.Interface{
			Spec: corev1alpha1.InterfaceSpec{Name: "ethernet-1/1.100"},
		},
	})
	if err != nil {
		t.Fatalf("EnsureDHCPRelay() error = %v", err)
	}
	if captured == nil {
		t.Fatal("EnsureDHCPRelay() did not call Update")
	}
	if captured.Interface != "ethernet-1/1" || captured.SubinterfaceIndex != 100 {
		t.Errorf("relay = (%q, %d), want (ethernet-1/1, 100)", captured.Interface, captured.SubinterfaceIndex)
	}
}

func TestDeleteDHCPRelay(t *testing.T) {
	t.Run("normal path deletes the interface relay", func(t *testing.T) {
		var deleted []*DHCPRelay
		p := &Provider{client: &gnmiext.ClientMock{
			DeleteFunc: func(_ context.Context, els ...gnmiext.DataElement) error {
				for _, el := range els {
					if r, ok := el.(*DHCPRelay); ok {
						deleted = append(deleted, r)
					}
				}
				return nil
			},
		}}

		err := p.DeleteDHCPRelay(t.Context(), &provider.DeleteDHCPRelayRequest{
			DHCPRelay: &corev1alpha1.DHCPRelay{},
			Interface: &corev1alpha1.Interface{Spec: corev1alpha1.InterfaceSpec{Name: "ethernet-1/1.100"}},
		})
		if err != nil {
			t.Fatalf("DeleteDHCPRelay() error = %v", err)
		}
		if len(deleted) != 1 || deleted[0].Interface != "ethernet-1/1" || deleted[0].SubinterfaceIndex != 100 {
			t.Fatalf("deleted = %+v, want one relay for ethernet-1/1 subinterface 100", deleted)
		}
	})

	t.Run("deprecated InterfaceRefs path is a no-op and does not panic", func(t *testing.T) {
		// Mock with no funcs: any device call panics, and req.Interface is nil.
		p := &Provider{client: &gnmiext.ClientMock{}}

		err := p.DeleteDHCPRelay(t.Context(), &provider.DeleteDHCPRelayRequest{
			DHCPRelay: &corev1alpha1.DHCPRelay{
				Spec: corev1alpha1.DHCPRelaySpec{
					InterfaceRefs: []corev1alpha1.LocalObjectReference{{Name: "eth1"}}, //nolint:staticcheck
				},
			},
		})
		if err != nil {
			t.Fatalf("DeleteDHCPRelay() error = %v", err)
		}
	})
}
