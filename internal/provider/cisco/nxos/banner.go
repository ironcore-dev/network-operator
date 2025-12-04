// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*Banner)(nil)
	_ gnmiext.Defaultable  = (*Banner)(nil)
)

// Banner represents the pre-login banner configuration of the device.
type Banner struct {
	// The value of the delimiter used to start and end the banner message
	Delimiter string `json:"delimiter"`
	// String to be displayed as the banner message
	Message string `json:"message"`
	// Type indicates whether this is a pre-login or post-login banner.
	// This field is not serialized to JSON and is only used internally
	// to determine the correct XPath for the banner configuration.
	Type BannerType `json:"-"`
}

func (b *Banner) XPath() string {
	if b.Type == PostLogin {
		return "System/userext-items/postloginbanner-items"
	}
	return "System/userext-items/preloginbanner-items"
}

func (b *Banner) Default() {
	b.Delimiter = "#"
	if b.Type == PreLogin {
		b.Message = "User Access Verification\n"
	}
}

type BannerType string

const (
	PreLogin  BannerType = "prelogin"
	PostLogin BannerType = "postlogin"
)

func BannerTypeFrom(bt v1alpha1.BannerType) (BannerType, error) {
	switch bt {
	case v1alpha1.BannerTypePreLogin:
		return PreLogin, nil
	case v1alpha1.BannerTypePostLogin:
		return PostLogin, nil
	default:
		return "", fmt.Errorf("unknown banner type: %s", bt)
	}
}
