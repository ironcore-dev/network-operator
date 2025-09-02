// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// This package enables the configuration of pre-login banner settings on the device.
//
// The pre-login banner is maintained with to the following YANG model:
//   - System/userext-items/preloginbanner-items
//

package banner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Banner)(nil)

type Banner struct {
	// The value of the delimiter used to start and end the banner message
	Delimiter string
	// String to be displayed as the banner message
	Message string
}

func (b *Banner) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	lines := strings.Split(b.Message, "\n")
	if len(lines) > 40 {
		return nil, errors.New("banner: maximum of 40 lines allowed")
	}
	for i, line := range lines {
		if n := utf8.RuneCountInString(line); n > 80 {
			return nil, fmt.Errorf("banner: line %d exceeds 80 characters (%d)", i+1, n)
		}
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/userext-items/preloginbanner-items",
			Value: &nxos.Cisco_NX_OSDevice_System_UserextItems_PreloginbannerItems{
				Delimiter: ygot.String(b.Delimiter),
				Message:   ygot.String(b.Message),
			},
		},
	}, nil
}

func (v *Banner) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	banner := &nxos.Cisco_NX_OSDevice_System_UserextItems_PreloginbannerItems{}
	banner.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/userext-items/preloginbanner-items",
			Value: banner,
		},
	}, nil
}
