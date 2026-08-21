// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ gnmiext.DataElement = (*RoutePolicy)(nil)

type RoutePolicy struct {
	Name string `json:"route-policy-name"`
	Body string `json:"rpl-route-policy"`
}

func (rp *RoutePolicy) XPath() string {
	return "Cisco-IOS-XR-policy-repository-cfg:routing-policy/route-policies/route-policy[route-policy-name=" + rp.Name + "]"
}

func NewRoutePolicy(vrf string) RoutePolicy {
	name := fmt.Sprintf("RPL_%s_IN", vrf)
	return RoutePolicy{
		Name: name,
		Body: fmt.Sprintf("route-policy %s\n  pass\nend-policy\n", name),
	}
}
