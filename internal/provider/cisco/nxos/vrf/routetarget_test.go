// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package vrf

import (
	"errors"
	"testing"
)

func TestNewRouteTarget_Defaults(t *testing.T) {
	addr, err := NewVPNIPv4Address(AFType0, "65000:100")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	rt, err := NewRouteTarget(*addr)
	if err != nil {
		t.Fatalf("NewRouteTarget returned error: %v", err)
	}
	if rt.addr.String() != "as2-nn2:65000:100" {
		t.Errorf("expected addr 'as2-nn2:65000:100', got %q", rt.addr.String())
	}
	if rt.action != RTNone {
		t.Errorf("expected default action RTNone, got %v", rt.action)
	}
	if rt.addressFamilyIPv4 {
		t.Errorf("expected addressFamilyIPv4 to be false by default")
	}
	if rt.addressFamilyIPv6 {
		t.Errorf("expected addressFamilyIPv6 to be false by default")
	}
	if rt.addEVPN {
		t.Errorf("expected addEVPN to be false by default")
	}
}

func TestNewRouteTarget_WithOptions(t *testing.T) {
	addr, err := NewVPNIPv4Address(AFType1, "10.0.0.1:200")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	rt, err := NewRouteTarget(
		*addr,
		WithAction(RTImport),
		WithAddressFamilyIPv4Unicast(true),
		WithAddressFamilyIPv6Unicast(true),
		WithEVPN(true),
	)
	if err != nil {
		t.Fatalf("NewRouteTarget returned error: %v", err)
	}
	if rt.action != RTImport {
		t.Errorf("expected action RTImport, got %v", rt.action)
	}
	if !rt.addressFamilyIPv4 {
		t.Errorf("expected addressFamilyIPv4 to be true")
	}
	if !rt.addressFamilyIPv6 {
		t.Errorf("expected addressFamilyIPv6 to be true")
	}
	if !rt.addEVPN {
		t.Errorf("expected addEVPN to be true")
	}
}

func TestNewRouteTarget_InvalidOption(t *testing.T) {
	addr, err := NewVPNIPv4Address(AFType0, "1:1")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	badOpt := func(rt *RouteTarget) error {
		return errors.New("test")
	}
	_, err = NewRouteTarget(*addr, badOpt)
	if err == nil {
		t.Errorf("expected error from bad option, got nil")
	}
}
