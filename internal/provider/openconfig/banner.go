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

var _ provider.BannerProvider = (*Provider)(nil)

func (p *Provider) EnsureBanner(ctx context.Context, req *provider.EnsureBannerRequest) error {
	t, err := toBannerType(req.Type)
	if err != nil {
		return err
	}
	return p.client.Update(ctx, &Banner{Type: t, Message: req.Message})
}

func (p *Provider) DeleteBanner(ctx context.Context, req *provider.DeleteBannerRequest) error {
	t, err := toBannerType(req.Type)
	if err != nil {
		return err
	}
	return p.client.Delete(ctx, &Banner{Type: t})
}

// BannerType represents the OpenConfig banner leaf type.
type BannerType string

const (
	BannerTypeLogin BannerType = "login-banner"
	BannerTypeMOTD  BannerType = "motd-banner"
)

func toBannerType(t v1alpha1.BannerType) (BannerType, error) {
	switch t {
	case v1alpha1.BannerTypePreLogin:
		return BannerTypeLogin, nil
	case v1alpha1.BannerTypePostLogin:
		return BannerTypeMOTD, nil
	default:
		return "", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.type",
			Description: fmt.Sprintf("unsupported banner type %q", t),
		})
	}
}

// Compile-time assertions.
var _ gnmiext.DataElement = (*Banner)(nil)

// Banner targets a single banner leaf in the system config.
type Banner struct {
	// Type is used only for XPath construction.
	Type    BannerType `json:"-"`
	Message string     `json:"-"`
}

func (b *Banner) XPath() string {
	return fmt.Sprintf("openconfig-system:system/config/%s", b.Type)
}

func (b *Banner) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Message)
}

func (b *Banner) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &b.Message)
}
