// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*VPCIf)(nil)

type VPCIf struct {
	ID             int `json:"id"`
	RsvpcConfItems struct {
		TDn string `json:"tDn"`
	} `json:"rsvpcConf-items"`
}

func (v *VPCIf) IsListItem() {}

func (v *VPCIf) XPath() string {
	return "System/vpc-items/inst-items/dom-items/if-items/If-list[id=" + strconv.Itoa(v.ID) + "]"
}

func (v *VPCIf) SetPortChannel(name string) {
	v.RsvpcConfItems.TDn = "/System/intf-items/aggr-items/AggrIf-list[id='" + name + "']"
}

type VPCIfItems struct {
	IfList []*VPCIf `json:"If-list"`
}

func (*VPCIfItems) XPath() string {
	return "System/vpc-items/inst-items/dom-items/if-items"
}

func (v *VPCIfItems) GetListItemByInterface(name string) *VPCIf {
	for _, item := range v.IfList {
		if item.RsvpcConfItems.TDn == "/System/intf-items/aggr-items/AggrIf-list[id='"+name+"']" {
			return item
		}
	}
	return nil
}
