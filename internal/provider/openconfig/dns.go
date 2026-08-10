// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.DNSProvider = (*Provider)(nil)

func (p *Provider) EnsureDNS(ctx context.Context, req *provider.EnsureDNSRequest) error {
	spec := req.DNS.Spec
	if err := validateDNSSpec(spec); err != nil {
		return err
	}

	d := &DNS{
		Config: &DNSConfig{
			Search: []string{spec.Domain},
		},
	}

	if len(spec.Servers) > 0 {
		d.Servers = &DNSServers{}
		for _, s := range spec.Servers {
			d.Servers.Server.Set(&DNSServer{
				Address: s.Address,
				Config:  &DNSServerConfig{Address: s.Address},
			})
		}
	}

	return p.client.Update(ctx, d)
}

func (p *Provider) DeleteDNS(ctx context.Context) error {
	return p.client.Delete(ctx, &DNS{})
}

func validateDNSSpec(spec v1alpha1.DNSSpec) error {
	var violations []apistatus.FieldViolation
	if spec.AdminState == v1alpha1.AdminStateDown {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.adminState",
			Description: "adminState Down is not supported by the OpenConfig DNS model",
		})
	}
	if spec.SourceInterfaceName != "" {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.sourceInterfaceName",
			Description: "sourceInterfaceName is not supported by the OpenConfig DNS model",
		})
	}
	for _, s := range spec.Servers {
		if s.VrfName != "" {
			violations = append(violations, apistatus.FieldViolation{
				Field:       fmt.Sprintf("spec.servers[%s].vrfName", s.Address),
				Description: "vrfName is not supported by the OpenConfig DNS model",
			})
		}
	}
	if len(violations) > 0 {
		return apistatus.NewUnsupportedFieldError(violations...)
	}
	return nil
}

// Compile-time assertions.
var _ gnmiext.DataElement = (*DNS)(nil)

// DNS represents the OpenConfig /system/dns container.
type DNS struct {
	Config  *DNSConfig  `json:"config,omitempty"`
	Servers *DNSServers `json:"servers,omitempty"`
}

func (*DNS) XPath() string { return "openconfig-system:system/dns" }

// DNSConfig holds the config container for DNS.
type DNSConfig struct {
	Search []string `json:"search,omitempty"`
}

// DNSServers holds the servers container for DNS.
type DNSServers struct {
	Server gnmiext.List[string, *DNSServer] `json:"server,omitempty"`
}

// DNSServer represents a single DNS server entry.
type DNSServer struct {
	Address string           `json:"address"`
	Config  *DNSServerConfig `json:"config,omitempty"`
}

func (s *DNSServer) Key() string { return s.Address }

// DNSServerConfig holds the config container for a DNS server.
type DNSServerConfig struct {
	Address string `json:"address"`
}
