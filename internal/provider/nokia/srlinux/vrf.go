// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.VRFProvider = (*Provider)(nil)

const DefaultNetworkInstance = "default"

// EnsureVRF implements [provider.VRFProvider].
func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	spec := req.VRF.Spec

	if spec.RouteDistinguisher != "" {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeDistinguisher",
			Description: "nokia/srlinux provider does not support route-distinguisher on SRLinux",
		})
	}
	if len(spec.RouteTargets) > 0 {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeTargets",
			Description: "nokia/srlinux provider does not support route-targets on SRLinux",
		})
	}
	if spec.VNI > 0 { //nolint:staticcheck
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.vni",
			Description: "nokia/srlinux provider does not support VNI on SRLinux, use EVPNInstance instead",
		})
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

	return p.client.Update(ctx, vrf)
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
