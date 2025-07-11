// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// This package enables the configuration of DNS settings on the device as per the following configuration sample:
//   - ip domain-lookup
//   - ip domain-name foo.bar
//   - ip name-server 10.10.10.10 use-vrf management
//
// Which corresponds to the following YANG model:
//   - System/dns-items/adminSt
//   - System/dns-items/prof-items/Prof-list[name=default]/dom-items
//   - System/dns-items/prof-items/Prof-list[name=default]/vrf-items/Vrf-list[name=management]/prov-items/Provider-list[addr=10.10.10.10]
//
// Implementation notes:
//   - We use a single prof-list under the key "default" for all providers. Multiple prof-lists are currently not supported.
//   - All three paths above share the prefix "System/dns-items", allowing using a single struct to represent the entire configuration. We
//     opt to use this approach instead of splitting the tree and setting multiple strucsts. Such approach would require more complex code
//     to ensure that we delete unused nodes in the same tree.
package dns

import (
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*DNS)(nil)

type DNS struct {
	// The value of the domain name equivalent to the CLI command "ip domain-name $DomainName".
	DomainName string
	// A list of DNS providers
	Providers []*Provider
	// True if DNS should be configured on the device. If false then the configuration is removed and
	// the other fields will be ignored.
	Enable bool
}

type Provider struct {
	// The address of the DNS provider
	Addr string
	// The VRF to use to reach the DNS provider
	Vrf string
	// The source interface to use to reach the DNS provider
	SrcIf string
}

func (d *DNS) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	if !d.Enable {
		return []gnmiext.Update{
			gnmiext.EditingUpdate{
				XPath: "System/dns-items",
				Value: &nxos.Cisco_NX_OSDevice_System_DnsItems{AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled},
			},
		}, nil
	}

	vrfs := &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems{
		VrfList: make(map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList, len(d.Providers)),
	}
	for _, prov := range d.Providers {
		err := vrfs.GetOrCreateVrfList(prov.Vrf).GetOrCreateProvItems().AppendProviderList(&nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList_ProvItems_ProviderList{
			Addr:  ygot.String(prov.Addr),
			SrcIf: ygot.String(prov.SrcIf),
		})
		if err != nil {
			return nil, err
		}
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/dns-items",
			Value: &nxos.Cisco_NX_OSDevice_System_DnsItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
				ProfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems{
					ProfList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList{
						"default": {
							Name:     ygot.String("default"),
							DomItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_DomItems{Name: ygot.String(d.DomainName)},
							VrfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems{VrfList: vrfs.VrfList},
						},
					},
				},
			},
		},
	}, nil
}

func (v *DNS) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	dns := new(nxos.Cisco_NX_OSDevice_System_DnsItems)
	dns.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/dns-items",
			Value: dns,
		},
	}, nil
}
