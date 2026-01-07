// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

const (
	BGPDefaultInstance = "default"
)

var _ gnmiext.DataElement = (*BGP)(nil)

type BGP struct {
	InstanceName string   `json:"instance-name"`
	AS           []ASList `json:"as"`
}

type ASList struct {
	ASNumber string `json:"as-number"`
}

func (p *BGP) XPath() string {
	return "Cisco-IOS-XR-um-router-bgp-cfg:router/bgp/instances/instance[instance-name=default]"
}
