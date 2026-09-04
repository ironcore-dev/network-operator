// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.NTPProvider = (*Provider)(nil)

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) error {
	spec := req.NTP.Spec
	if err := validateNTPSpec(spec); err != nil {
		return err
	}

	n := &NTP{
		Config: &NTPConfig{
			Enabled: spec.AdminState == v1alpha1.AdminStateUp,
		},
	}

	if len(spec.Servers) > 0 {
		n.Servers = &NTPServers{}
		for _, s := range spec.Servers {
			networkInstance := s.VrfName

			n.Servers.Server.Set(&NTPServer{
				Address: s.Address,
				Config: &NTPServerConfig{
					Address:         s.Address,
					Prefer:          s.Prefer,
					NetworkInstance: networkInstance,
					SourceAddress:   spec.SourceAddress,
				},
			})
		}
	}

	return p.client.Update(ctx, n)
}

func (p *Provider) DeleteNTP(ctx context.Context) error {
	return p.client.Delete(ctx, &NTP{})
}

func validateNTPSpec(spec v1alpha1.NTPSpec) error {
	var violations []apistatus.FieldViolation
	if spec.SourceInterfaceName != "" {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.sourceInterfaceName",
			Description: "sourceInterfaceName is not supported by the OpenConfig NTP model. Use sourceAddress instead.",
		})
	}
	if len(violations) > 0 {
		return apistatus.NewUnsupportedFieldError(violations...)
	}
	return nil
}

// Compile-time assertions.
var _ gnmiext.DataElement = (*NTP)(nil)

// DNS represents the OpenConfig /system/ntp container.
type NTP struct {
	Config  *NTPConfig  `json:"config"`
	Servers *NTPServers `json:"servers"`
}

func (*NTP) XPath() string { return "openconfig-system:system/ntp" }

// NTPConfig holds the config container for NTP.
type NTPConfig struct {
	Enabled bool `json:"enabled"`
}

// NTPServers holds the servers container for NTP.
type NTPServers struct {
	Server gnmiext.List[string, *NTPServer] `json:"server"`
}

// NTPServer represents a single NTP server entry.
type NTPServer struct {
	Address string           `json:"address"`
	Config  *NTPServerConfig `json:"config"`
}

func (s *NTPServer) Key() string { return s.Address }

// NTPServerConfig holds the config container for a NTP server.
type NTPServerConfig struct {
	Address         string `json:"address"`
	Prefer          bool   `json:"prefer,omitempty"`
	NetworkInstance string `json:"network-instance,omitempty"` // Maps to VrfName
	SourceAddress   string `json:"source-address,omitempty"`
}
