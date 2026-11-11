// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"net/netip"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

// captureSet runs fn against a provider whose client records every queued Set
// element, and returns them for inspection.
func captureSet(t *testing.T, fn func(p *Provider) error) []gnmiext.DataElement {
	t.Helper()
	var captured []gnmiext.DataElement
	mock := &gnmiext.ClientMock{
		DoFunc: func(_ context.Context, b *gnmiext.SetBuilder) error {
			b.Split(func(el gnmiext.DataElement) bool {
				captured = append(captured, el)
				return true
			})
			return nil
		},
		UpdateFunc: func(_ context.Context, elems ...gnmiext.DataElement) error {
			captured = append(captured, elems...)
			return nil
		},
		DeleteFunc: func(_ context.Context, elems ...gnmiext.DataElement) error {
			captured = append(captured, elems...)
			return nil
		},
	}
	if err := fn(&Provider{client: mock}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return captured
}

// findIface returns the first captured interface config, or fails.
func findIface(t *testing.T, els []gnmiext.DataElement) *ifaceConfig {
	t.Helper()
	for _, el := range els {
		if i, ok := el.(*ifaceConfig); ok {
			return i
		}
	}
	t.Fatal("no ifaceConfig queued")
	return nil
}

// findSub returns the first captured subinterface element, or fails.
func findSub(t *testing.T, els []gnmiext.DataElement) *subinterface {
	t.Helper()
	for _, el := range els {
		if s, ok := el.(*subinterface); ok {
			return s
		}
	}
	t.Fatal("no subinterface queued")
	return nil
}

func interfaceReq(spec v1alpha1.InterfaceSpec) *provider.EnsureInterfaceRequest {
	return &provider.EnsureInterfaceRequest{Interface: &v1alpha1.Interface{Spec: spec}}
}

func TestEnsureInterfacePhysicalIPv4(t *testing.T) {
	req := interfaceReq(v1alpha1.InterfaceSpec{
		Name:        "ethernet-1/1",
		Type:        v1alpha1.InterfaceTypePhysical,
		AdminState:  v1alpha1.AdminStateUp,
		Description: "uplink",
		MTU:         9000,
	})
	req.IPv4 = provider.IPv4AddressList{netip.MustParsePrefix("10.0.0.1/31")}

	els := captureSet(t, func(p *Provider) error { return p.EnsureInterface(t.Context(), req) })
	iface := findIface(t, els)
	if iface.Name != "ethernet-1/1" || iface.AdminState != adminEnable || iface.Description != "uplink" || iface.MTU != 9000 {
		t.Fatalf("unexpected interface config: %+v", iface)
	}
	sub := findSub(t, els)
	if sub.ParentName != "ethernet-1/1" || sub.Index != 0 || sub.Type != subIfTypeRouted {
		t.Fatalf("unexpected subinterface: %+v", sub)
	}
	if _, ok := sub.IPv4.Address.Get("10.0.0.1/31"); !ok {
		t.Fatalf("missing IPv4 address, got %+v", sub.IPv4)
	}
}

func TestEnsureInterfaceLoopbackNoType(t *testing.T) {
	req := interfaceReq(v1alpha1.InterfaceSpec{
		Name:       "lo0",
		Type:       v1alpha1.InterfaceTypeLoopback,
		AdminState: v1alpha1.AdminStateUp,
	})
	req.IPv4 = provider.IPv4AddressList{netip.MustParsePrefix("10.0.1.10/32")}
	req.IPv6 = provider.IPv6AddressList{netip.MustParsePrefix("2001:db8::1/128")}

	sub := findSub(t, captureSet(t, func(p *Provider) error { return p.EnsureInterface(t.Context(), req) }))
	if sub.Type != "" {
		t.Fatalf("loopback subinterface must not set type, got %q", sub.Type)
	}
	if _, ok := sub.IPv4.Address.Get("10.0.1.10/32"); !ok {
		t.Fatal("missing IPv4 address")
	}
	if _, ok := sub.IPv6.Address.Get("2001:db8::1/128"); !ok {
		t.Fatal("missing IPv6 address")
	}
}

func TestEnsureInterfaceAggregateMultiChassis(t *testing.T) {
	id := int16(42)
	req := interfaceReq(v1alpha1.InterfaceSpec{
		Name:       "lag1",
		Type:       v1alpha1.InterfaceTypeAggregate,
		AdminState: v1alpha1.AdminStateUp,
		Aggregation: &v1alpha1.Aggregation{
			ControlProtocol: v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
		},
	})
	req.MultiChassisID = &id

	iface := findIface(t, captureSet(t, func(p *Provider) error { return p.EnsureInterface(t.Context(), req) }))
	if iface.LAG == nil || iface.LAG.LagType != lagTypeLACP || iface.LAG.LACP == nil {
		t.Fatalf("unexpected lag config: %+v", iface.LAG)
	}
	if iface.LAG.LACP.LACPMode != lacpModeActive || iface.LAG.LACP.AdminKey != 42 {
		t.Fatalf("unexpected lacp config: %+v", iface.LAG.LACP)
	}
}

func TestEnsureSubinterfaceDot1Q(t *testing.T) {
	req := interfaceReq(v1alpha1.InterfaceSpec{
		Name:               "ethernet-1/2.100",
		Type:               v1alpha1.InterfaceTypeSubinterface,
		AdminState:         v1alpha1.AdminStateUp,
		ParentInterfaceRef: &v1alpha1.LocalObjectReference{Name: "ethernet-1/2"},
		Encapsulation:      &v1alpha1.Encapsulation{Type: v1alpha1.EncapsulationTypeDot1Q, Tag: 100},
	})
	req.IPv4 = provider.IPv4AddressList{netip.MustParsePrefix("10.10.0.1/24")}

	els := captureSet(t, func(p *Provider) error { return p.EnsureInterface(t.Context(), req) })

	var gotTagging bool
	var sub *subinterface
	for _, el := range els {
		switch v := el.(type) {
		case *vlanTagging:
			if v.Name == "ethernet-1/2" && v.Value {
				gotTagging = true
			}
		case *subinterface:
			sub = v
		}
	}
	if !gotTagging {
		t.Fatal("parent vlan-tagging was not patched")
	}
	if sub == nil || sub.ParentName != "ethernet-1/2" || sub.Index != 100 || sub.Type != subIfTypeRouted {
		t.Fatalf("unexpected subinterface: %+v", sub)
	}
	if sub.VLAN == nil || sub.VLAN.Encap == nil || sub.VLAN.Encap.SingleTagged == nil || sub.VLAN.Encap.SingleTagged.VLANID != 100 {
		t.Fatalf("unexpected vlan encap: %+v", sub.VLAN)
	}
}

func TestEnsureInterfaceBFD(t *testing.T) {
	mult := int32(3)
	req := interfaceReq(v1alpha1.InterfaceSpec{
		Name:       "ethernet-1/3",
		Type:       v1alpha1.InterfaceTypePhysical,
		AdminState: v1alpha1.AdminStateUp,
		BFD: &v1alpha1.BFD{
			Enabled:                  true,
			DesiredMinimumTxInterval: &metav1.Duration{Duration: 250 * time.Millisecond},
			DetectionMultiplier:      &mult,
		},
	})
	req.IPv4 = provider.IPv4AddressList{netip.MustParsePrefix("10.0.0.5/31")}

	els := captureSet(t, func(p *Provider) error { return p.EnsureInterface(t.Context(), req) })

	var bfd *bfdSubinterface
	for _, el := range els {
		if b, ok := el.(*bfdSubinterface); ok {
			bfd = b
		}
	}
	if bfd == nil {
		t.Fatal("no bfd subinterface queued")
	}
	if bfd.ID != "ethernet-1/3.0" || bfd.AdminState != adminEnable || bfd.DesiredMinTxInterval != 250000 || bfd.DetectionMultiplier != 3 {
		t.Fatalf("unexpected bfd config: %+v", bfd)
	}
}

func TestEnsureInterfaceUnsupported(t *testing.T) {
	tests := []struct {
		name  string
		spec  v1alpha1.InterfaceSpec
		field string
	}{
		{
			name:  "routed vlan",
			spec:  v1alpha1.InterfaceSpec{Name: "irb0", Type: v1alpha1.InterfaceTypeRoutedVLAN},
			field: "spec.type",
		},
		{
			name:  "switchport",
			spec:  v1alpha1.InterfaceSpec{Name: "ethernet-1/4", Type: v1alpha1.InterfaceTypePhysical, Switchport: &v1alpha1.Switchport{Mode: v1alpha1.SwitchportModeAccess}},
			field: "spec.switchport",
		},
		{
			name:  "fec",
			spec:  v1alpha1.InterfaceSpec{Name: "ethernet-1/5", Type: v1alpha1.InterfaceTypePhysical, Ethernet: &v1alpha1.Ethernet{FECMode: v1alpha1.FECModeRS528}},
			field: "spec.ethernet.fecMode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{}
			err := p.EnsureInterface(t.Context(), interfaceReq(tt.spec))
			se, ok := apistatus.FromError(err)
			if !ok || se.Code != apistatus.CodeUnsupportedField {
				t.Fatalf("want UnsupportedField error, got %v", err)
			}
			if len(se.FieldViolations) == 0 || se.FieldViolations[0].Field != tt.field {
				t.Fatalf("want field %q, got %+v", tt.field, se.FieldViolations)
			}
		})
	}
}

func TestDeleteInterface(t *testing.T) {
	phys := interfaceReq(v1alpha1.InterfaceSpec{Name: "ethernet-1/1", Type: v1alpha1.InterfaceTypePhysical})
	els := captureSet(t, func(p *Provider) error {
		return p.DeleteInterface(t.Context(), &provider.DeleteInterfaceRequest{Interface: phys.Interface})
	})
	iface := findIface(t, els)
	if iface.AdminState != adminDisable {
		t.Fatalf("physical delete must disable the port, got %q", iface.AdminState)
	}

	lo := interfaceReq(v1alpha1.InterfaceSpec{Name: "lo0", Type: v1alpha1.InterfaceTypeLoopback})
	els = captureSet(t, func(p *Provider) error {
		return p.DeleteInterface(t.Context(), &provider.DeleteInterfaceRequest{Interface: lo.Interface})
	})
	if findIface(t, els).Name != "lo0" {
		t.Fatal("loopback delete did not target lo0")
	}
}

func TestBuildIPv4Unnumbered(t *testing.T) {
	v4 := buildIPv4(provider.IPv4Unnumbered{SourceInterface: "system0.0"})
	if v4 == nil || v4.Unnumbered == nil || v4.Unnumbered.Interface != "system0.0" || v4.Unnumbered.AdminState != adminEnable {
		t.Fatalf("unexpected unnumbered config: %+v", v4)
	}
	if len(v4.Address) != 0 {
		t.Fatal("unnumbered must not carry addresses")
	}
}

func TestBuildIPv6LinkLocalOnly(t *testing.T) {
	v6 := buildIPv6(provider.IPv6LinkLocalOnly{})
	if v6 == nil || v6.AdminState != adminEnable {
		t.Fatalf("link-local-only must enable ipv6, got %+v", v6)
	}
	if len(v6.Address) != 0 {
		t.Fatal("link-local-only must not carry global addresses")
	}
}

func TestEncapToVLANQinQ(t *testing.T) {
	vlan, err := encapToVLAN(&v1alpha1.Encapsulation{Type: v1alpha1.EncapsulationTypeQinQ, OuterTag: 200, InnerTag: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dt := vlan.Encap.DoubleTagged
	if dt == nil || dt.OuterVLANID != 200 || dt.InnerVLANID != 10 {
		t.Fatalf("unexpected double-tagged encap: %+v", dt)
	}
}

func TestParseSubinterfaceName(t *testing.T) {
	parent, index, err := parseSubinterfaceName("ethernet-1/2.100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parent != "ethernet-1/2" || index != 100 {
		t.Fatalf("want ethernet-1/2 index 100, got %q index %d", parent, index)
	}
	if _, _, err := parseSubinterfaceName("ethernet-1/2"); err == nil {
		t.Fatal("want error for name without index")
	}
}
