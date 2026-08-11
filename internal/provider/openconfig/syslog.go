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

var _ provider.SyslogProvider = (*Provider)(nil)

func (p *Provider) EnsureSyslog(ctx context.Context, req *provider.EnsureSyslogRequest) error {
	spec := req.Syslog.Spec

	if len(spec.Facilities) > 0 {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.facilities",
			Description: "openconfig provider does not support syslog facilities on SRLinux",
		})
	}

	sb := new(gnmiext.SetBuilder)

	for _, srv := range spec.Servers {
		if srv.VrfName != "" {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.servers[].vrfName",
				Description: "openconfig provider does not support syslog VRF on SRLinux",
			})
		}

		rs := &SyslogRemoteServer{
			Host: srv.Address,
			Config: &SyslogRemoteServerConfig{
				Host:       srv.Address,
				RemotePort: uint16(srv.Port), //nolint:gosec
			},
			Selectors: &SyslogSelectors{},
		}
		rs.Selectors.Selector.Set(&SyslogSelector{
			Facility: SyslogFacilityAll,
			Severity: toSyslogSeverity(srv.Severity),
			Config: &SyslogSelectorConfig{
				Facility: SyslogFacilityAll,
				Severity: toSyslogSeverity(srv.Severity),
			},
		})
		sb.Update(rs)
	}

	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteSyslog(ctx context.Context) error {
	return p.client.Delete(ctx, &SyslogContainer{})
}

// SyslogSeverity represents the OpenConfig syslog severity identity.
type SyslogSeverity string

const (
	SyslogSeverityDebug         SyslogSeverity = "DEBUG"
	SyslogSeverityInformational SyslogSeverity = "INFORMATIONAL"
	SyslogSeverityNotice        SyslogSeverity = "NOTICE"
	SyslogSeverityWarning       SyslogSeverity = "WARNING"
	SyslogSeverityError         SyslogSeverity = "ERROR"
	SyslogSeverityCritical      SyslogSeverity = "CRITICAL"
	SyslogSeverityAlert         SyslogSeverity = "ALERT"
	SyslogSeverityEmergency     SyslogSeverity = "EMERGENCY"
)

func toSyslogSeverity(s v1alpha1.Severity) SyslogSeverity {
	switch s {
	case v1alpha1.SeverityDebug:
		return SyslogSeverityDebug
	case v1alpha1.SeverityInfo:
		return SyslogSeverityInformational
	case v1alpha1.SeverityNotice:
		return SyslogSeverityNotice
	case v1alpha1.SeverityWarning:
		return SyslogSeverityWarning
	case v1alpha1.SeverityError:
		return SyslogSeverityError
	case v1alpha1.SeverityCritical:
		return SyslogSeverityCritical
	case v1alpha1.SeverityAlert:
		return SyslogSeverityAlert
	case v1alpha1.SeverityEmergency:
		return SyslogSeverityEmergency
	default:
		return SyslogSeverityInformational
	}
}

// SyslogFacility represents the OpenConfig syslog facility identity.
type SyslogFacility string

const (
	SyslogFacilityAll SyslogFacility = "ALL"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*SyslogRemoteServer)(nil)
	_ gnmiext.DataElement = (*SyslogContainer)(nil)
)

// SyslogContainer targets the full logging container for deletion.
type SyslogContainer struct{}

func (*SyslogContainer) XPath() string {
	return "openconfig-system:system/logging/remote-servers"
}

// SyslogRemoteServer targets a specific remote-server entry.
type SyslogRemoteServer struct {
	Host      string                    `json:"-"`
	Config    *SyslogRemoteServerConfig `json:"config,omitempty"`
	Selectors *SyslogSelectors          `json:"selectors,omitempty"`
}

func (s *SyslogRemoteServer) XPath() string {
	return fmt.Sprintf("openconfig-system:system/logging/remote-servers/remote-server[host=%s]", s.Host)
}

// SyslogRemoteServerConfig holds the remote server config.
type SyslogRemoteServerConfig struct {
	Host       string `json:"host"`
	RemotePort uint16 `json:"remote-port,omitempty"`
}

// SyslogSelectors holds the selector list.
type SyslogSelectors struct {
	Selector gnmiext.List[string, *SyslogSelector] `json:"selector,omitempty"`
}

// SyslogSelector represents a facility+severity selector.
type SyslogSelector struct {
	Facility SyslogFacility        `json:"facility"`
	Severity SyslogSeverity        `json:"severity"`
	Config   *SyslogSelectorConfig `json:"config,omitempty"`
}

func (s *SyslogSelector) Key() string { return string(s.Facility) + "/" + string(s.Severity) }

// SyslogSelectorConfig holds selector config.
type SyslogSelectorConfig struct {
	Facility SyslogFacility `json:"facility"`
	Severity SyslogSeverity `json:"severity"`
}
