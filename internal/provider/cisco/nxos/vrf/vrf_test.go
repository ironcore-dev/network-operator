// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package vrf

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestNewVRF_Basic(t *testing.T) {
	vrf, err := NewVRF("test-vrf")
	if err != nil {
		t.Fatalf("NewVRF returned error: %v", err)
	}
	if vrf.name != "test-vrf" {
		t.Errorf("expected name 'test-vrf', got %q", vrf.name)
	}
	if vrf.vni != nil {
		t.Errorf("expected vni to be nil by default")
	}
	if vrf.routeDistinguiser != nil {
		t.Errorf("expected routeDistinguiser to be nil by default")
	}
	if len(vrf.routeTargets) != 0 {
		t.Errorf("expected routeTargets to be empty by default")
	}
}

func TestNewVRF_WithVNI(t *testing.T) {
	vrf, err := NewVRF("vrf-vni", WithVNI(10001, true))
	if err != nil {
		t.Fatalf("NewVRF returned error: %v", err)
	}
	if vrf.vni == nil {
		t.Fatalf("expected vni to be set")
	}
	if vrf.vni.id != 10001 {
		t.Errorf("expected vni.id 10001, got %d", vrf.vni.id)
	}
	if !vrf.vni.isL3 {
		t.Errorf("expected vni.isL3 to be true")
	}
}

func TestNewVRF_WithRouteDistinguisher(t *testing.T) {
	addr, err := NewVPNIPv4Address(AFType0, "65000:100")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	vrf, err := NewVRF("vrf-rd", WithRouteDistinguisher(*addr))
	if err != nil {
		t.Fatalf("NewVRF returned error: %v", err)
	}
	if vrf.routeDistinguiser == nil {
		t.Fatalf("expected routeDistinguiser to be set")
	}
	expectedRD, err := NewVPNIPv4Address(AFType0, "65000:100")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if *vrf.routeDistinguiser != *expectedRD {
		t.Errorf("expected routeDistinguiser to be %v, got %v", expectedRD, vrf.routeDistinguiser)
	}
}

func TestNewVRF_WithRouteTarget(t *testing.T) {
	// route target 1
	addr, err := NewVPNIPv4Address(AFType0, "65000:100")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	rt1, err := NewRouteTarget(*addr)
	if err != nil {
		t.Fatalf("failed to create RouteTarget: %v", err)
	}
	// route target 2
	addr, err = NewVPNIPv4Address(AFType1, "1.2.3.4:200")
	if err != nil {
		t.Fatalf("failed to create VPNIPv4Address: %v", err)
	}
	rt2, err := NewRouteTarget(*addr)
	if err != nil {
		t.Fatalf("failed to create RouteTarget: %v", err)
	}

	// create VRF with multiple route targets
	vrf, err := NewVRF("vrf-rt",
		WithRouteTarget(*rt1),
		WithRouteTarget(*rt2),
	)
	if err != nil {
		t.Fatalf("NewVRF returned error: %v", err)
	}

	rts := []RouteTarget{*rt1, *rt2}
	if !cmp.Equal(vrf.routeTargets, rts, cmp.AllowUnexported(RouteTarget{}, VPNIPv4Address{})) {
		t.Errorf("expected routeTargets to be %v, got %v", rts, vrf.routeTargets)
	}
}

func TestNewVRF_InvalidOption(t *testing.T) {
	badOpt := func(rt *VRF) error {
		return errors.New("test")
	}
	_, err := NewVRF("test", badOpt)
	if err == nil {
		t.Errorf("expected error from bad option, got nil")
	}
}

func TestNewVRF_ToYGOT(t *testing.T) {
	vrfCfg2 := &VRF{
		name: "mgmt2",
		vni:  &VNI{id: 3000, isL3: true},
		routeDistinguiser: &VPNIPv4Address{
			afType:         AFType0,
			administrator:  "10000",
			assignedNumber: 101,
		},
		routeTargets: []RouteTarget{
			{
				addr:              VPNIPv4Address{afType: AFType0, administrator: "65000", assignedNumber: 200},
				addressFamilyIPv4: true,
				action:            RTImport,
			},
			{
				addr:              VPNIPv4Address{afType: AFType0, administrator: "65000", assignedNumber: 220},
				addressFamilyIPv4: true,
				action:            RTBoth,
			},
			{
				addr:              VPNIPv4Address{afType: AFType1, administrator: "1.2.3.4", assignedNumber: 300},
				addressFamilyIPv4: true,
				addressFamilyIPv6: true,
				action:            RTBoth,
				addEVPN:           true,
			},
		},
	}
	updates, err := vrfCfg2.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("ToYGOT returned error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	update, ok := updates[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Fatalf("expected update to be of type gnmiext.ReplacingUpdate, got %T", updates[0])
	}
	if update.XPath != "System/inst-items/Inst-list[name="+vrfCfg2.name+"]" {
		t.Errorf("expected XPath to be 'System/inst-items/Inst-list[name=%s]', got %s", vrfCfg2.name, update.XPath)
	}
	ygotVRF, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_InstItems_InstList)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_InstItems_InstList")
	}
	if *ygotVRF.Name != vrfCfg2.name {
		t.Errorf("expected VRF name %s, got %s", vrfCfg2.name, *ygotVRF.Name)
	}

	if ygotVRF.L3Vni == nil || *ygotVRF.L3Vni != vrfCfg2.vni.isL3 {
		t.Errorf("expected L3Vni to be %v, got %v", vrfCfg2.vni.isL3, ygotVRF.L3Vni)
	}
	if ygotVRF.Encap == nil || *ygotVRF.Encap != "vxlan-3000" {
		t.Errorf("expected Encap to be 'vxlan-3000', got %s", *ygotVRF.Encap)
	}
	d := ygotVRF.GetDomItems().GetDomList(vrfCfg2.name)
	if d.Rd == nil || *d.Rd != "rd:as2-nn2:10000:101" {
		t.Errorf("expected Route Distinguisher to be 'rd:as2-nn2:10000:101', got %s", *d.Rd)
	}
	// count number of entries in the data model added by the route targets
	numRttPList := CountNodesOfType(
		ygotVRF.GetDomItems().GetDomList(vrfCfg2.name).GetAfItems(),
		"Cisco_NX_OSDevice_System_InstItems_InstList_DomItems_DomList_AfItems_DomAfList_CtrlItems_AfCtrlList_RttpItems_RttPList_EntItems_RttEntryList",
	)
	if numRttPList != 7 {
		t.Errorf("expected 7 RttEntryList nodes, got %d", numRttPList)
	}
	// check the route targets themselves
	for _, rt := range vrfCfg2.routeTargets {
		if rt.addressFamilyIPv4 {
			aftype := nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast
			if rt.addEVPN {
				aftype = nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn
			}
			if rt.action == RTImport || rt.action == RTBoth {
				importList := ygotVRF.GetDomItems().GetDomList(vrfCfg2.name).GetAfItems().
					GetDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
					GetCtrlItems().
					GetAfCtrlList(aftype).
					GetRttpItems().
					GetRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).
					GetEntItems().
					GetRttEntryList("route-target:" + rt.addr.String())
				if importList == nil {
					t.Errorf("expected import entry for %s, got nil", rt.addr.String())
				}
			}
			if rt.action == RTExport || rt.action == RTBoth {
				exportList := ygotVRF.GetDomItems().GetDomList(vrfCfg2.name).GetAfItems().
					GetDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
					GetCtrlItems().
					GetAfCtrlList(aftype).
					GetRttpItems().
					GetRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).
					GetEntItems().
					GetRttEntryList("route-target:" + rt.addr.String())
				if exportList == nil {
					t.Errorf("expected export entry for %s, got nil", rt.addr.String())
				}
			}
		}
		if rt.addressFamilyIPv6 {
			aftype := nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast
			if rt.addEVPN {
				aftype = nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn
			}
			if rt.action == RTImport || rt.action == RTBoth {
				importList := ygotVRF.GetDomItems().GetDomList(vrfCfg2.name).GetAfItems().
					GetDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
					GetCtrlItems().
					GetAfCtrlList(aftype).
					GetRttpItems().
					GetRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).
					GetEntItems().
					GetRttEntryList("route-target:" + rt.addr.String())
				if importList == nil {
					t.Errorf("expected import entry for %s, got nil", rt.addr.String())
				}
			}
			if rt.action == RTExport || rt.action == RTBoth {
				exportList := ygotVRF.GetDomItems().GetDomList(vrfCfg2.name).GetAfItems().
					GetDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
					GetCtrlItems().
					GetAfCtrlList(aftype).
					GetRttpItems().
					GetRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).
					GetEntItems().
					GetRttEntryList("route-target:" + rt.addr.String())
				if exportList == nil {
					t.Errorf("expected export entry for %s, got nil", rt.addr.String())
				}
			}
		}
	}
}

func CountNodesOfType(s any, targetTypeName string) int {
	count := 0
	var visit func(v reflect.Value)
	visit = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if !v.IsValid() {
			return
		}
		// Check for type match
		if v.Type().Name() == targetTypeName {
			count++
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := range v.NumField() {
				f := v.Field(i)
				// Only exported fields
				if v.Type().Field(i).PkgPath == "" {
					visit(f)
				}
			}
		case reflect.Slice:
			for i := range v.Len() {
				visit(v.Index(i))
			}
		case reflect.Map:
			keys := v.MapKeys()
			for i := range keys {
				visit(v.MapIndex(keys[i]))
			}
		}
	}
	visit(reflect.ValueOf(s))
	return count
}

func TestVRF_Reset(t *testing.T) {
	vrf := &VRF{
		name: "test-vrf",
	}

	updates, err := vrf.Reset(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	update, ok := updates[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Fatalf("expected update to be of type gnmiext.DeletingUpdate, got %T", updates[0])
	}
	if update.XPath != "System/inst-items/Inst-list[name=test-vrf]/" {
		t.Errorf("expected XPath to be 'System/inst-items/Inst-list[name=test-vrf]/', got %s", update.XPath)
	}
}
