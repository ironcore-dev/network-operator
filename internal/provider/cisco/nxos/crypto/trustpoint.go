// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"context"
	"errors"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Trustpoint)(nil)

type Trustpoint struct {
	ID string
}

var ErrAlreadyExists = errors.New("crypto: trustpoint already exists")

func (t *Trustpoint) ToYGOT(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	exists, err := client.Exists(ctx, "System/userext-items/pkiext-items/tp-items/TP-list[name="+t.ID+"]")
	if err != nil {
		return nil, fmt.Errorf("trustpoint: failed to get trustpoint %q: %w", t.ID, err)
	}
	if exists {
		// Trying to replace an existing trustpoint configuration will fail with "disassociating rsa key-pair not allowed when identity certificate exists"
		return nil, ErrAlreadyExists
	}
	v := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList{}
	v.PopulateDefaults()
	v.Name = ygot.String(t.ID)
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/userext-items/pkiext-items/tp-items/TP-list[name=" + t.ID + "]",
			Value: v,
		},
	}, nil
}

func (t *Trustpoint) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/userext-items/pkiext-items/tp-items/TP-list[name=" + t.ID + "]",
		},
		gnmiext.DeletingUpdate{
			XPath: "System/userext-items/pkiext-items/keyring-items/KeyRing-list[name=" + t.ID + "]",
		},
	}, nil
}
