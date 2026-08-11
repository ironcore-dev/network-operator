// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.VRFProvider = (*Provider)(nil)

func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	spec := req.VRF.Spec

	if spec.RouteDistinguisher != "" {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeDistinguisher",
			Description: "openconfig provider does not support route-distinguisher on SRLinux",
		})
	}
	if len(spec.RouteTargets) > 0 {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.routeTargets",
			Description: "openconfig provider does not support route-targets on SRLinux",
		})
	}
	if spec.VNI > 0 { //nolint:staticcheck
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.vni",
			Description: "openconfig provider does not support VNI on SRLinux",
		})
	}

	niType := NetworkInstanceTypeL3VRF
	if spec.Name == DefaultNetworkInstance {
		niType = NetworkInstanceTypeDefaultInstance
	}

	ni := &NetworkInstance{
		Name: spec.Name,
		Config: &NetworkInstanceConfig{
			Name:        spec.Name,
			Type:        niType,
			Description: spec.Description,
		},
	}

	return p.client.Update(ctx, ni)
}

func (p *Provider) DeleteVRF(ctx context.Context, req *provider.VRFRequest) error {
	ni := &NetworkInstance{Name: req.VRF.Spec.Name}
	return p.client.Delete(ctx, ni)
}

// NetworkInstanceType represents the OpenConfig network-instance type identity.
type NetworkInstanceType string

const (
	NetworkInstanceTypeL3VRF           NetworkInstanceType = "openconfig-network-instance-types:L3VRF"
	NetworkInstanceTypeDefaultInstance NetworkInstanceType = "openconfig-network-instance-types:DEFAULT_INSTANCE"
)

// Compile-time assertion.
var _ gnmiext.DataElement = (*NetworkInstance)(nil)

// NetworkInstance represents an OC network-instance list entry.
type NetworkInstance struct {
	Name   string                 `json:"-"`
	Config *NetworkInstanceConfig `json:"config,omitempty"`
}

func (ni *NetworkInstance) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]", ni.Name)
}

// NetworkInstanceConfig holds config for a network-instance.
type NetworkInstanceConfig struct {
	Name        string              `json:"name"`
	Type        NetworkInstanceType `json:"type"`
	Description string              `json:"description,omitempty"`
}
