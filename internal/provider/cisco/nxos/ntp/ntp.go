// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ntp

import (
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*NTP)(nil)

type NTP struct {
	EnableLogging bool
	SrcInterface  string
	Servers       []*Server
}

type Server struct {
	Name      string
	Preferred bool
	Vrf       string
	Key       uint16
}

var ignorePaths = []string{
	"/adminSt",
	"/passive",
	"/ownerKey",
	"/authSt",
	"/loggingLevel",
	"/allowControl",
	"/allowPrivate",
	"/masterStratum",
	"/master",
	"/ownerTag",
	"/rateLimit",
}

func (n *NTP) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	updates := make([]gnmiext.Update, 0, len(n.Servers)+1)

	prov := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{NtpProviderList: make(map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList, len(n.Servers))}
	for _, s := range n.Servers {
		l := prov.GetOrCreateNtpProviderList(s.Name)
		l.PopulateDefaults()
		l.KeyId = ygot.Uint16(s.Key)
		l.Name = ygot.String(s.Name)
		l.Preferred = ygot.Bool(s.Preferred)
		l.ProvT = nxos.Cisco_NX_OSDevice_Datetime_ProvT_server
		l.Vrf = ygot.String(s.Vrf)

		// The `ygot.Diff` method generates multiple `Update` objects, one for each attribute.
		// Since `ProvT` is a mandatory field, it must be included whenever a new server is created.
		// However, the current implementation does not guarantee that `ProvT` is the first attribute in the updates,
		// which can cause the update to fail.
		// To address this limitation, we create a separate `Update` specifically for the `ProvT` attribute
		// and prepend it to the list of updates.
		update := gnmiext.EditingUpdate{
			XPath: "System/time-items/prov-items/NtpProvider-list[name=" + s.Name + "]",
			Value: &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
				ProvT: nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
			},
		}
		updates = append(updates, update)
	}

	logging := nxos.Cisco_NX_OSDevice_Datetime_AdminState_disabled
	if n.EnableLogging {
		logging = nxos.Cisco_NX_OSDevice_Datetime_AdminState_enabled
	}

	// append the updates containing all attributes of the NTP server
	return append(updates, gnmiext.EditingUpdate{
		XPath: "System/time-items",
		Value: &nxos.Cisco_NX_OSDevice_System_TimeItems{
			SrcIfItems: &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String(n.SrcInterface)},
			Logging:    logging,
			ProvItems:  prov,
		},
		IgnorePaths: ignorePaths,
	}), nil
}

func (v *NTP) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	ntp := &nxos.Cisco_NX_OSDevice_System_TimeItems{}
	ntp.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/time-items",
			Value: ntp,
		},
	}, nil
}
