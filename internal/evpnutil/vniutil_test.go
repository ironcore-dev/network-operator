package evpnutil

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

func TestComputeVNIRouteTarget(t *testing.T) {
	buildFabricAndClaim := func(t *testing.T, asn intstr.IntOrString, value string) (*evpnv1alpha1.Fabric, *poolv1alpha1.Claim) {
		t.Helper()
		fabric := &evpnv1alpha1.Fabric{}
		claim := &poolv1alpha1.Claim{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(map[string]any{
			"spec": map[string]any{
				"overlay": map[string]any{
					"ibgp": map[string]any{
						"asNumber": asn.String(),
					},
				},
			},
		}, fabric); err != nil {
			t.Fatalf("convert Fabric fixture: %v", err)
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(map[string]any{
			"status": map[string]any{
				"value": value,
			},
		}, claim); err != nil {
			t.Fatalf("convert Claim fixture: %v", err)
		}

		return fabric, claim
	}

	t.Run("encodes RFC 8365 route targets", func(t *testing.T) {
		tests := []struct {
			name    string
			asn     intstr.IntOrString
			value   string
			wantVNI uint64
			wantRT  string
		}{
			{name: "integer ASN and lowest VNI", asn: intstr.FromInt(1), value: "1", wantVNI: uint64(1), wantRT: "1:268435457"},
			{name: "string ASN and highest VNI", asn: intstr.FromString("65535"), value: "16777214", wantVNI: uint64(16777214), wantRT: "65535:285212670"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				fabric, claim := buildFabricAndClaim(t, tt.asn, tt.value)
				vni, rt, err := ComputeVNIRouteTarget(claim, fabric, claim)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if vni != tt.wantVNI {
					t.Fatalf("vni = %d, want %d", vni, tt.wantVNI)
				}
				if rt != tt.wantRT {
					t.Fatalf("rt = %q, want %q", rt, tt.wantRT)
				}
			})
		}
	})

	t.Run("classifies invalid inputs", func(t *testing.T) {
		tests := []struct {
			name string
			asn  intstr.IntOrString
			vni  string
		}{
			{name: "zero ASN", asn: intstr.FromInt(0), vni: "1"},
			{name: "four-byte ASN", asn: intstr.FromInt(65536), vni: "1"},
			{name: "dotted ASN", asn: intstr.FromString("1.1"), vni: "1"},
			{name: "zero VNI", asn: intstr.FromInt(65000), vni: "0"},
			{name: "reserved VNI", asn: intstr.FromInt(65000), vni: "16777215"},
			{name: "VNI overflow", asn: intstr.FromInt(65000), vni: "16777216"},
			{name: "malformed VNI", asn: intstr.FromInt(65000), vni: "invalid"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				fabric, claim := buildFabricAndClaim(t, tt.asn, tt.vni)
				vni, rt, err := ComputeVNIRouteTarget(claim, fabric, claim)
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if vni != 0 {
					t.Fatalf("vni = %d, want 0", vni)
				}
				if rt != "" {
					t.Fatalf("rt = %q, want empty", rt)
				}
			})
		}
	})
}
