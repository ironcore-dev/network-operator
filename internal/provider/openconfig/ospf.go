// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var _ provider.OSPFProvider = (*Provider)(nil)

func (p *Provider) EnsureOSPF(_ context.Context, _ *provider.EnsureOSPFRequest) error {
	return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
		Field:       "spec",
		Description: "openconfig provider does not support OSPF configuration on SRLinux",
	})
}

func (p *Provider) DeleteOSPF(_ context.Context, _ *provider.DeleteOSPFRequest) error {
	return nil
}

func (p *Provider) GetOSPFStatus(_ context.Context, _ *provider.OSPFStatusRequest) (provider.OSPFStatus, error) {
	return provider.OSPFStatus{}, nil
}
