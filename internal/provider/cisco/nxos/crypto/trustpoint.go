// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Trustpoint)(nil)

type Trustpoint struct {
	ID string
}

func (t *Trustpoint) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
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

func (t *Trustpoint) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/userext-items/pkiext-items/tp-items/TP-list[name=" + t.ID + "]",
		},
	}, nil
}
