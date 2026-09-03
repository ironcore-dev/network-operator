// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.NTPProvider = (*Provider)(nil)

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) error {
	spec := req.NTP.Spec

	n := &NTP{
		Config: &NTPConfig{
			Enabled: spec.AdminState == v1alpha1.AdminStateUp,
		},
	}

	// TODO(AdamT): handle spec.SourceInterfaceName mapping to openconfig

	if len(spec.Servers) > 0 {
		n.Servers = &NTPServers{}
		for _, s := range spec.Servers {
			n.Servers.Server.Set(&NTPServer{
				Address: s.Address,
				Config: &NTPServerConfig{
					Prefer:          s.Prefer,
					NetworkInstance: s.VrfName,
				},
			})
		}
	}

	return p.client.Update(ctx, n)
}

func (p *Provider) DeleteNTP(ctx context.Context) error {
	return p.client.Delete(ctx, &NTP{})
}

// Compile-time assertions.
var _ gnmiext.DataElement = (*NTP)(nil)

// DNS represents the OpenConfig /system/ntp container.
type NTP struct {
	Config  *NTPConfig  `json:"config,omitempty"`
	Servers *NTPServers `json:"servers,omitempty"`
}

func (*NTP) XPath() string { return "openconfig-system:system/ntp" }

// NTPConfig holds the config container for NTP.
type NTPConfig struct {
	Enabled bool `json:"enabled"` // Maps to AdminState
}

// NTPServers holds the servers container for NTP.
type NTPServers struct {
	Server gnmiext.List[string, *NTPServer] `json:"server,omitempty"`
}

// NTPServer represents a single NTP server entry.
type NTPServer struct {
	Address string           `json:"address"`
	Config  *NTPServerConfig `json:"config,omitempty"`
}

func (s *NTPServer) Key() string { return s.Address }

// NTPServerConfig holds the config container for a NTP server.
type NTPServerConfig struct {
	Address         string `json:"address"`
	Prefer          bool   `json:"prefer,omitempty"`
	NetworkInstance string `json:"network-instance,omitempty"` // Maps to VrfName
}
