// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"fmt"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.VRFProvider = (*Provider)(nil)

const DefaultNetworkInstance = "default"

// EnsureVRF implements [provider.VRFProvider].
func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	spec := req.VRF.Spec

	if spec.RouteDistinguisher == v1alpha1.RouteDistinguisherAuto {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeDistinguisher",
			Description: "nokia/srlinux provider does not support automatic route-distinguisher derivation",
		})
	}
	if spec.Name == DefaultNetworkInstance && (spec.RouteDistinguisher != "" || len(spec.RouteTargets) > 0) {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeDistinguisher",
			Description: "nokia/srlinux provider does not support bgp-vpn on the default network instance",
		})
	}
	if spec.VNI > 0 { //nolint:staticcheck
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.vni",
			Description: "nokia/srlinux provider does not support VNI on SRLinux, use EVPNInstance instead",
		})
	}

	importRT, exportRT := "", ""
	for _, rt := range spec.RouteTargets {
		if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
			if importRT != "" && "target:"+rt.Value != importRT {
				return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
					Field:       "spec.routeTargets",
					Description: "nokia/srlinux provider supports at most one distinct import route target value",
				})
			}
			importRT = "target:" + rt.Value
		}
		if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
			if exportRT != "" && "target:"+rt.Value != exportRT {
				return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
					Field:       "spec.routeTargets",
					Description: "nokia/srlinux provider supports at most one distinct export route target value",
				})
			}
			exportRT = "target:" + rt.Value
		}
	}

	vrfType := NetworkInstanceTypeIPVRF
	if spec.Name == DefaultNetworkInstance {
		vrfType = NetworkInstanceTypeDefault
	}

	vrf := &VRF{
		Name:        spec.Name,
		Type:        vrfType,
		Description: spec.Description,
		AdminState:  AdminStEnable,
	}
	b := new(gnmiext.SetBuilder).Update(vrf)
	if spec.RouteDistinguisher != "" || importRT != "" || exportRT != "" {
		bgpvpn := &BGPVPNInstance{NetworkInstanceName: spec.Name}
		if spec.RouteDistinguisher != "" {
			bgpvpn.RouteDistinguisher = &BGPVPNRouteDistinguisher{RD: spec.RouteDistinguisher}
		}
		if importRT != "" || exportRT != "" {
			bgpvpn.RouteTarget = &BGPVPNRouteTarget{ImportRT: importRT, ExportRT: exportRT}
		}
		b.Update(bgpvpn)
	}
	return p.client.Do(ctx, b)
}

// DeleteVRF implements [provider.VRFProvider].
func (p *Provider) DeleteVRF(ctx context.Context, req *provider.DeleteVRFRequest) error {
	vrf := &VRF{
		Name: req.VRF.Spec.Name,
	}
	return p.client.Delete(ctx, vrf)
}

type NetworkInstanceType string

const (
	NetworkInstanceTypeIPVRF   NetworkInstanceType = "srl_nokia-network-instance:ip-vrf"
	NetworkInstanceTypeDefault NetworkInstanceType = "srl_nokia-network-instance:default"
)

var _ gnmiext.DataElement = (*VRF)(nil)

type VRF struct {
	Name        string              `json:"-"`
	Type        NetworkInstanceType `json:"type"`
	Description string              `json:"description,omitempty"`
	AdminState  AdminSt             `json:"admin-state"`
}

func (vrf *VRF) XPath() string {
	return fmt.Sprintf("srl_nokia-network-instance:network-instance[name=%s]", vrf.Name)
}

var _ gnmiext.DataElement = (*BGPVPNInstance)(nil)

type BGPVPNInstance struct {
	NetworkInstanceName string                    `json:"-"`
	RouteDistinguisher  *BGPVPNRouteDistinguisher `json:"route-distinguisher,omitempty"`
	RouteTarget         *BGPVPNRouteTarget        `json:"route-target,omitempty"`
}

func (b *BGPVPNInstance) XPath() string {
	return fmt.Sprintf(
		"srl_nokia-network-instance:network-instance[name=%s]/protocols/srl_nokia-bgp-vpn:bgp-vpn/bgp-instance[id=1]",
		b.NetworkInstanceName,
	)
}

type BGPVPNRouteDistinguisher struct {
	RD string `json:"rd"`
}

type BGPVPNRouteTarget struct {
	ImportRT string `json:"import-rt,omitempty"`
	ExportRT string `json:"export-rt,omitempty"`
}
