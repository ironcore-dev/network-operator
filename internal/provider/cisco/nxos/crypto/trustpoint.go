// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

type Trustpoint struct {
	ID string
}

var _ gnmiext.DeviceConf = Trustpoints{}

type Trustpoints []*Trustpoint

func (t Trustpoints) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	items := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems{TPList: make(map[string]*nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList, len(t))}
	for _, tp := range t {
		list := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList{
			Name: ygot.String(tp.ID),
		}
		list.PopulateDefaults()

		if err := items.AppendTPList(list); err != nil {
			return nil, err
		}
	}

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/userext-items/pkiext-items/tp-items",
			Value: items,
		},
	}, nil
}

func (t Trustpoints) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	items := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems{}
	items.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/userext-items/pkiext-items/tp-items",
			Value: items,
		},
	}, nil
}
