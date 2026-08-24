// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.LLDPProvider = (*Provider)(nil)

func (p *Provider) EnsureLLDP(ctx context.Context, req *provider.LLDPRequest) error {
	spec := req.LLDP.Spec

	sb := new(gnmiext.SetBuilder)
	sb.Update(&LLDPConfig{
		Enabled: spec.AdminState == v1alpha1.AdminStateUp,
	})

	for i, ref := range spec.InterfaceRefs {
		if i >= len(req.Interfaces) {
			break
		}
		sb.Update(&LLDPInterfaceConfig{
			Name:    req.Interfaces[i].Spec.Name,
			Enabled: ref.AdminState == v1alpha1.AdminStateUp,
		})
	}

	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteLLDP(ctx context.Context, req *provider.LLDPRequest) error {
	// LLDP cannot be fully deleted on SRLinux — disable it.
	return p.client.Update(ctx, &LLDPConfig{Enabled: false})
}

func (p *Provider) GetLLDPStatus(ctx context.Context, _ *provider.LLDPRequest) (provider.LLDPStatus, error) {
	state := &LLDPState{}
	if err := p.client.GetState(ctx, state); err != nil {
		return provider.LLDPStatus{}, err
	}
	return provider.LLDPStatus{OperStatus: state.Enabled}, nil
}

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*LLDPConfig)(nil)
	_ gnmiext.DataElement = (*LLDPInterfaceConfig)(nil)
	_ gnmiext.DataElement = (*LLDPState)(nil)
)

// LLDPConfig targets openconfig-lldp:lldp/config.
type LLDPConfig struct {
	Enabled bool `json:"enabled"`
}

func (*LLDPConfig) XPath() string {
	return "openconfig-lldp:lldp/config"
}

// LLDPInterfaceConfig targets a per-interface LLDP config.
type LLDPInterfaceConfig struct {
	Name    string `json:"-"`
	Enabled bool   `json:"enabled"`
}

func (l *LLDPInterfaceConfig) XPath() string {
	return fmt.Sprintf("openconfig-lldp:lldp/interfaces/interface[name=%s]/config", l.Name)
}

// LLDPState reads lldp/state for oper status.
type LLDPState struct {
	Enabled bool `json:"enabled"`
}

func (*LLDPState) XPath() string {
	return "openconfig-lldp:lldp/state"
}
