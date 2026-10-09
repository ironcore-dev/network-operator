// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ gnmiext.DataElement = (*Interfaces)(nil)

type Interfaces struct {
	Interface []*Interface `json:"srl_nokia-interfaces:interface"`
}

func (*Interfaces) XPath() string {
	return "srl_nokia-interfaces:interface"
}

type Interface struct {
	Name     string    `json:"name"`
	Ethernet *Ethernet `json:"ethernet"`
}

type Ethernet struct {
	PortSpeed string `json:"port-speed"`
}
