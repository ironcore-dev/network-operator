// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package term

import (
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var (
	_ gnmiext.DeviceConf = (*Console)(nil)
	_ gnmiext.DeviceConf = (*VTY)(nil)
)

// Console represents the primary terminal line configuration.
type Console struct {
	// Maximum time allowed for a command to execute in minutes.
	// Leave unspecified (zero) to disable.
	Timeout uint32
}

func (c *Console) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/terml-items/ln-items/cons-items/execTmeout-items",
			Value: &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_ConsItems_ExecTmeoutItems{
				Timeout: ygot.Uint32(c.Timeout),
			},
		},
	}, nil
}

func (v *Console) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	cons := &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_ConsItems{}
	cons.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/terml-items/ln-items/cons-items",
			Value: cons,
		},
	}, nil
}

// VTY represents the virtual terminal line configuration.
type VTY struct {
	// IPv4 access control list to be applied for packets.
	ACL string
	// Maximum number of concurrent vsh sessions.
	SessionLimit uint32
	// Maximum time allowed for a command to execute in minutes.
	// Leave unspecified (zero) to disable.
	Timeout uint32
}

func (v *VTY) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	u := []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/terml-items/ln-items/vty-items",
			Value: &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems{
				ExecTmeoutItems: &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems_ExecTmeoutItems{Timeout: ygot.Uint32(v.Timeout)},
				SsLmtItems:      &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems_SsLmtItems{SesLmt: ygot.Uint32(v.SessionLimit)},
			},
			IgnorePaths: []string{
				"/absTmeout-items",
				"/lgoutWarning-items",
			},
		},
	}

	if v.ACL != "" {
		u = append(u, gnmiext.EditingUpdate{
			XPath: "System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items",
			Value: &nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_PolicyItems_IngressItems_VtyItems_AclItems{Name: ygot.String(v.ACL)},
		})
	}

	return u, nil
}

func (v *VTY) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	vtys := &nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems{}
	vtys.PopulateDefaults()
	acls := &nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_PolicyItems_IngressItems_VtyItems_AclItems{}
	acls.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/terml-items/ln-items/vty-items",
			Value: vtys,
		},
		gnmiext.EditingUpdate{
			XPath: "System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items",
			Value: acls,
		},
	}, nil
}
