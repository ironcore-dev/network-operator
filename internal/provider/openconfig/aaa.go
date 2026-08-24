// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.AAAProvider = (*Provider)(nil)

func (p *Provider) EnsureAAA(ctx context.Context, req *provider.EnsureAAARequest) error {
	spec := req.AAA.Spec

	if spec.Authorization != nil {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.authorization",
			Description: "openconfig provider does not support AAA authorization on SRLinux",
		})
	}

	sb := new(gnmiext.SetBuilder)

	for _, sg := range spec.ServerGroups {
		if sg.Type == v1alpha1.AAAServerGroupTypeRADIUS {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.serverGroups[].type",
				Description: "openconfig provider does not support RADIUS server groups on SRLinux",
			})
		}

		group := &AAAServerGroup{
			Name: sg.Name,
			Config: &AAAServerGroupConfig{
				Name: sg.Name,
				Type: AAAServerGroupTypeTACACS,
			},
			Servers: &AAAServers{},
		}

		for _, srv := range sg.Servers {
			s := &AAAServer{
				Address: srv.Address,
				Config: &AAAServerConfig{
					Address: srv.Address,
				},
			}
			if srv.Timeout != nil {
				s.Config.Timeout = uint16(srv.Timeout.Seconds())
			}
			if srv.TACACS != nil {
				key := req.TACACSServerKeys[srv.Address]
				s.TACACS = &AAAServerTACACS{
					Config: &AAAServerTACACSConfig{
						Port:      uint16(srv.TACACS.Port), //nolint:gosec
						SecretKey: key,
					},
				}
			}
			group.Servers.Server.Set(s)
		}
		sb.Update(group)
	}

	if spec.Authentication != nil {
		methods := make([]AAAMethodType, 0, len(spec.Authentication.Methods))
		for _, m := range spec.Authentication.Methods {
			methods = append(methods, toAAAMethod(m))
		}
		sb.Update(&AAAAuthenticationConfig{Methods: methods})
	}

	if spec.Accounting != nil {
		methods := make([]AAAMethodType, 0, len(spec.Accounting.Methods))
		for _, m := range spec.Accounting.Methods {
			methods = append(methods, toAAAMethod(m))
		}
		sb.Update(&AAAAccountingConfig{Methods: methods})
	}

	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteAAA(ctx context.Context, req *provider.DeleteAAARequest) error {
	sb := new(gnmiext.SetBuilder)
	for _, sg := range req.AAA.Spec.ServerGroups {
		sb.Delete(&AAAServerGroup{Name: sg.Name})
	}
	if len(req.AAA.Spec.ServerGroups) == 0 {
		sb.Delete(&AAAContainer{})
	}
	return p.client.Do(ctx, sb)
}

func toAAAMethod(m v1alpha1.AAAMethod) AAAMethodType {
	switch m.Type {
	case v1alpha1.AAAMethodTypeLocal:
		return AAAMethodTypeLocal
	case v1alpha1.AAAMethodTypeNone:
		return AAAMethodTypeNone
	case v1alpha1.AAAMethodTypeGroup:
		return AAAMethodType(m.GroupName)
	default:
		return AAAMethodTypeLocal
	}
}

// AAAServerGroupType represents the OpenConfig AAA server group type identity.
type AAAServerGroupType string

const (
	AAAServerGroupTypeTACACS AAAServerGroupType = "openconfig-aaa:TACACS"
)

// AAAMethodType represents the AAA authentication/accounting method string.
type AAAMethodType string

const (
	AAAMethodTypeLocal AAAMethodType = "local"
	AAAMethodTypeNone  AAAMethodType = "none"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*AAAServerGroup)(nil)
	_ gnmiext.DataElement = (*AAAAuthenticationConfig)(nil)
	_ gnmiext.DataElement = (*AAAAccountingConfig)(nil)
	_ gnmiext.DataElement = (*AAAContainer)(nil)
)

// AAAContainer targets the full AAA container for deletion.
type AAAContainer struct{}

func (*AAAContainer) XPath() string { return "openconfig-system:system/aaa" }

// AAAServerGroup targets a server-group entry.
type AAAServerGroup struct {
	Name    string                `json:"-"`
	Config  *AAAServerGroupConfig `json:"config,omitempty"`
	Servers *AAAServers           `json:"servers,omitempty"`
}

func (g *AAAServerGroup) XPath() string {
	return fmt.Sprintf("openconfig-system:system/aaa/server-groups/server-group[name=%s]", g.Name)
}

// AAAServerGroupConfig holds the server-group config.
type AAAServerGroupConfig struct {
	Name string             `json:"name"`
	Type AAAServerGroupType `json:"type"`
}

// AAAServers holds the server list.
type AAAServers struct {
	Server gnmiext.List[string, *AAAServer] `json:"server,omitempty"`
}

// AAAServer represents a single server entry.
type AAAServer struct {
	Address string           `json:"address"`
	Config  *AAAServerConfig `json:"config,omitempty"`
	TACACS  *AAAServerTACACS `json:"tacacs,omitempty"`
}

func (s *AAAServer) Key() string { return s.Address }

// AAAServerConfig holds the server config.
type AAAServerConfig struct {
	Address string `json:"address"`
	Timeout uint16 `json:"timeout,omitempty"`
}

// AAAServerTACACS holds the tacacs container.
type AAAServerTACACS struct {
	Config *AAAServerTACACSConfig `json:"config,omitempty"`
}

// AAAServerTACACSConfig holds tacacs config.
// SecretKey is write-only — the device returns an encrypted form that
// would never match the plaintext, so we exclude it from unmarshal to
// avoid perpetual diffs.
type AAAServerTACACSConfig struct {
	Port      uint16 `json:"port,omitempty"`
	SecretKey string `json:"secret-key,omitempty"`
}

func (c *AAAServerTACACSConfig) UnmarshalJSON(data []byte) error {
	type alias struct {
		Port uint16 `json:"port,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.Port = a.Port
	return nil
}

// AAAAuthenticationConfig targets aaa/authentication/config.
type AAAAuthenticationConfig struct {
	Methods []AAAMethodType `json:"authentication-method"`
}

func (*AAAAuthenticationConfig) XPath() string {
	return "openconfig-system:system/aaa/authentication/config"
}

// AAAAccountingConfig targets aaa/accounting/config.
type AAAAccountingConfig struct {
	Methods []AAAMethodType `json:"accounting-method"`
}

func (*AAAAccountingConfig) XPath() string {
	return "openconfig-system:system/aaa/accounting/config"
}
