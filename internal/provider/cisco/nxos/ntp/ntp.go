// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ntp

import (
	"context"

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
}

func (n *NTP) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	prov := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{NtpProviderList: make(map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList, len(n.Servers))}
	for _, s := range n.Servers {
		l := prov.GetOrCreateNtpProviderList(s.Name)
		l.PopulateDefaults()
		l.Name = ygot.String(s.Name)
		l.Preferred = ygot.Bool(s.Preferred)
		l.ProvT = nxos.Cisco_NX_OSDevice_Datetime_ProvT_server
		l.Vrf = ygot.String(s.Vrf)
	}

	logging := nxos.Cisco_NX_OSDevice_Datetime_AdminState_disabled
	if n.EnableLogging {
		logging = nxos.Cisco_NX_OSDevice_Datetime_AdminState_enabled
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/ntpd-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NtpdItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/time-items",
			Value: &nxos.Cisco_NX_OSDevice_System_TimeItems{
				SrcIfItems: &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String(n.SrcInterface)},
				Logging:    logging,
				ProvItems:  prov,
			},
		},
	}, nil
}

// Reset disables the NTP feature and resets the configuration to the default values.
func (n *NTP) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	ntp := &nxos.Cisco_NX_OSDevice_System_TimeItems{}
	ntp.PopulateDefaults()
	ntp.AdminSt = nxos.Cisco_NX_OSDevice_Datetime_AdminState_disabled
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/ntpd-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NtpdItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/time-items",
			Value: ntp,
		},
	}, nil
}
