// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func TestAsnToUint32(t *testing.T) {
	tests := []struct {
		name string
		asn  intstr.IntOrString
		want uint32
	}{
		{"int value", intstr.FromInt32(65000), 65000},
		{"string plain", intstr.FromString("4294967295"), 4294967295},
		{"string dotted", intstr.FromString("1.1"), 65537},
		{"string dotted large", intstr.FromString("65535.65535"), 4294967295},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := asnToUint32(test.asn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Errorf("asnToUint32(%v) = %d, want %d", test.asn, got, test.want)
			}
		})
	}
}

func TestDeleteBGPPeer_Unnumbered(t *testing.T) {
	// The mock has no functions set, so any device call panics.
	p := newProviderWithClient(&gnmiext.ClientMock{})

	err := p.DeleteBGPPeer(t.Context(), &provider.DeleteBGPPeerRequest{
		BGPPeer: &v1alpha1.BGPPeer{
			Spec: v1alpha1.BGPPeerSpec{
				InterfaceRef: &v1alpha1.LocalObjectReference{Name: "eth1-1"},
			},
		},
		PeerInterface: "ethernet-1/1",
	})
	if err != nil {
		t.Fatalf("DeleteBGPPeer() error = %v", err)
	}
}
