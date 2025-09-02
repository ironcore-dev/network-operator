// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package vrf

import (
	"context"
	"strconv"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*VRF)(nil)

// VRF represents a Virtual Routing and Forwarding instance or context as per Cisco definition
type VRF struct {
	// name is the display name of the VRF.
	name string
	// vni is the Virtual Network Identifier for the VRF.
	vni *VNI
	// rd is the Route Distinguisher for the VRF.
	routeDistinguiser *VPNIPv4Address
	// rts is a list of Route Targets associated with the VRF.
	routeTargets []RouteTarget
}

type VNI struct {
	id   uint32
	isL3 bool
}

type VRFOption func(*VRF) error

// NewVRF creates a new VRF configuration with the given name.
func NewVRF(name string, opts ...VRFOption) (*VRF, error) {
	v := &VRF{
		name: name,
	}
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func WithVNI(vni uint32, isL3 bool) VRFOption {
	return func(v *VRF) error {
		v.vni = &VNI{
			id:   vni,
			isL3: isL3,
		}
		return nil
	}
}

// WithRouteDistinguisher sets the Route Distinguisher for the VRF. If set multiple times,
// the last one will be used.
// // `addr“ must be constructed with `NewVPNIPv4Address
func WithRouteDistinguisher(addr VPNIPv4Address) VRFOption {
	return func(v *VRF) error {
		v.routeDistinguiser = &addr
		return nil
	}
}

// WithRouteTarget adds a Route Target to the VRF. Repeated elements will be added to the list
// but will have no effect in the API calls towards the device.
func WithRouteTarget(rt RouteTarget) VRFOption {
	return func(v *VRF) error {
		v.routeTargets = append(v.routeTargets, rt)
		return nil
	}
}

// ToYGOT converts the VRF configuration to a YANG model representation for gNMI. It returns a slice of Updates of type ReplacingUpdate.
// The entire tree under "System/inst-items/Inst-list[vrf-name]" will be replaced ("vrf-name" is the value used during the VRF initialization).
func (v *VRF) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	val := &nxos.Cisco_NX_OSDevice_System_InstItems_InstList{
		Name: ygot.String(v.name),
	}
	if v.vni != nil {
		val.L3Vni = ygot.Bool(v.vni.isL3)
		val.Encap = ygot.String("vxlan-" + strconv.FormatUint(uint64(v.vni.id), 10))
	}
	if v.routeDistinguiser != nil {
		d := val.GetOrCreateDomItems().GetOrCreateDomList(v.name)
		d.Rd = ygot.String("rd:" + v.routeDistinguiser.String())
	}

	if len(v.routeTargets) > 0 {
		// pre-fetch the branches containing the list of route targets for both IPv4 and IPv6 unicast
		// TODO: this approach creates empty lists, which might not be wanted or needed
		v4RTItems := val.GetOrCreateDomItems().
			GetOrCreateDomList(v.name).
			GetOrCreateAfItems().
			GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
			GetOrCreateCtrlItems().
			GetOrCreateAfCtrlList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
			GetOrCreateRttpItems()
		v4EvpnRTItems := val.GetOrCreateDomItems().
			GetOrCreateDomList(v.name).
			GetOrCreateAfItems().
			GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv4_ucast).
			GetOrCreateCtrlItems().
			GetOrCreateAfCtrlList(nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn).
			GetOrCreateRttpItems()
		v6RTItems := val.GetOrCreateDomItems().
			GetOrCreateDomList(v.name).
			GetOrCreateAfItems().
			GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast).
			GetOrCreateCtrlItems().
			GetOrCreateAfCtrlList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast).
			GetOrCreateRttpItems()
		v6EvpnRTItems := val.GetOrCreateDomItems().
			GetOrCreateDomList(v.name).
			GetOrCreateAfItems().
			GetOrCreateDomAfList(nxos.Cisco_NX_OSDevice_Bgp_AfT_ipv6_ucast).
			GetOrCreateCtrlItems().
			GetOrCreateAfCtrlList(nxos.Cisco_NX_OSDevice_Bgp_AfT_l2vpn_evpn).
			GetOrCreateRttpItems()
		v4ImportEntries := v4RTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).GetOrCreateEntItems()
		v4ExportEntries := v4RTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).GetOrCreateEntItems()
		v6ImportEntries := v6RTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).GetOrCreateEntItems()
		v6ExportEntries := v6RTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).GetOrCreateEntItems()
		v4EvpnImportEntries := v4EvpnRTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).GetOrCreateEntItems()
		v4EvpnExportEntries := v4EvpnRTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).GetOrCreateEntItems()
		v6EvpnImportEntries := v6EvpnRTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_import).GetOrCreateEntItems()
		v6EvpnExportEntries := v6EvpnRTItems.GetOrCreateRttPList(nxos.Cisco_NX_OSDevice_Rtctrl_RttPType_export).GetOrCreateEntItems()
		// iterate over the route targets and create the entries in the appropriate lists
		for _, rt := range v.routeTargets {
			key := "route-target:" + rt.addr.String()
			if rt.addressFamilyIPv4 {
				if rt.addEVPN && (rt.action == RTImport || rt.action == RTBoth) {
					v4EvpnImportEntries.GetOrCreateRttEntryList(key)
				}
				if rt.addEVPN && (rt.action == RTExport || rt.action == RTBoth) {
					v4EvpnExportEntries.GetOrCreateRttEntryList(key)
				}
				if !rt.addEVPN && (rt.action == RTImport || rt.action == RTBoth) {
					v4ImportEntries.GetOrCreateRttEntryList(key)
				}
				if !rt.addEVPN && (rt.action == RTExport || rt.action == RTBoth) {
					v4ExportEntries.GetOrCreateRttEntryList(key)
				}
			}
			if rt.addressFamilyIPv6 {
				if rt.addEVPN && (rt.action == RTImport || rt.action == RTBoth) {
					v6EvpnImportEntries.GetOrCreateRttEntryList(key)
				}
				if rt.addEVPN && (rt.action == RTExport || rt.action == RTBoth) {
					v6EvpnExportEntries.GetOrCreateRttEntryList(key)
				}
				if !rt.addEVPN && (rt.action == RTImport || rt.action == RTBoth) {
					v6ImportEntries.GetOrCreateRttEntryList(key)
				}
				if !rt.addEVPN && (rt.action == RTExport || rt.action == RTBoth) {
					v6ExportEntries.GetOrCreateRttEntryList(key)
				}
			}
		}
	}

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/inst-items/Inst-list[name=" + v.name + "]",
			Value: val,
		},
	}, nil
}

// Reset returns DeletingUpdates for the path "System/inst-items/Inst-list[vrf-name]", where
// "vrf-name" is the value used during the VRF initialization.
func (v *VRF) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/inst-items/Inst-list[name=" + v.name + "]/",
		},
	}, nil
}
