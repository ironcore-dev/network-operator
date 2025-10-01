// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"
	"regexp"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

type PhysIf struct {
	Name         string        `json:"-"`
	Description  string        `json:"description"`
	Active       string        `json:"active"`
	Vrf          string        `json:"Cisco-IOS-XR-infra-rsi-cfg:vrf,omitempty"`
	Statistics   Statistics    `json:"Cisco-IOS-XR-infra-statsd-cfg:statistics,omitempty"`
	IPv4Network  IPv4Network   `json:"Cisco-IOS-XR-ipv4-io-cfg:ipv4-network,omitempty"`
	IPv6Network  IPv6Network   `json:"Cisco-IOS-XR-ipv6-ma-cfg:ipv6-network,omitempty"`
	IPv6Neighbor IPv6Neighbor  `json:"Cisco-IOS-XR-ipv6-nd-cfg:ipv6-neighbor,omitempty"`
	MTUs         MTUs          `json:"mtus,omitempty"`
	Shutdown     gnmiext.Empty `json:"shutdown,omitempty"`
}

type Statistics struct {
	LoadInterval uint8 `json:"load-interval"`
}

type IPv4Network struct {
	Addresses AddressesIPv4 `json:"addresses"`
	Mtu       uint16        `json:"mtu"`
}

type AddressesIPv4 struct {
	Primary Primary `json:"primary"`
}

type Primary struct {
	Address string `json:"address"`
	Netmask string `json:"netmask"`
}

type IPv6Network struct {
	Mtu       uint16        `json:"mtu"`
	Addresses AddressesIPv6 `json:"addresses"`
}

type AddressesIPv6 struct {
	RegularAddresses RegularAddresses `json:"regular-addresses"`
}

type RegularAddresses struct {
	RegularAddress []RegularAddress `json:"regular-address"`
}

type RegularAddress struct {
	Address      string `json:"address"`
	PrefixLength uint8  `json:"prefix-length"`
	Zone         string `json:"zone"`
}

type IPv6Neighbor struct {
	RASuppress bool `json:"ra-suppress"`
}

type MTUs struct {
	MTU []MTU `json:"mtu"`
}

type MTU struct {
	MTU   int32  `json:"mtu"`
	Owner string `json:"owner"`
}

func (i *PhysIf) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration[active=act][interface-name=%s]", i.Name)
}

func (i *PhysIf) String() string {
	return fmt.Sprintf("Name: %s, Description=%s, ShutDown=%t", i.Name, i.Description, i.Shutdown)
}

type IFaceSpeed string

const (
	Speed10G  IFaceSpeed = "TenGigE"
	Speed25G  IFaceSpeed = "TwentyFiveGigE"
	Speed40G  IFaceSpeed = "FortyGigE"
	Speed100G IFaceSpeed = "HundredGigE"
)

func ExtractMTUOwnerFromIfaceName(ifaceName string) (IFaceSpeed, error) {
	// Match the port_type in an interface name <port_type>/<rack>/<slot/<module>/<port>
	// E.g. match TwentyFiveGigE of interface with name TwentyFiveGigE0/0/0/1
	re := regexp.MustCompile(`^\D*`)

	mtuOwner := string(re.Find([]byte(ifaceName)))

	if mtuOwner == "" {
		return "", fmt.Errorf("failed to extract MTU owner from interface name %s", ifaceName)
	}

	switch mtuOwner {
	case string(Speed10G):
		return Speed10G, nil
	case string(Speed25G):
		return Speed25G, nil
	case string(Speed40G):
		return Speed25G, nil
	case string(Speed100G):
		return Speed100G, nil
	default:
		return "", fmt.Errorf("unsupported interface type %s for MTU owner extraction", mtuOwner)
	}
}

type PhysIfStateType string

const (
	StateUp        PhysIfStateType = "im-state-up"
	StateDown      PhysIfStateType = "im-state-down"
	StateNotReady  PhysIfStateType = "im-state-not-ready"
	StateAdminDown PhysIfStateType = "im-state-admin-down"
	StateShutDown  PhysIfStateType = "im-state-shutdown"
)

type PhysIfState struct {
	State string `json:"state"`
	Name  string `json:"-"`
}

func (phys *PhysIfState) XPath() string {
	// (fixme): hardcoded route processor for the moment
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-oper:interface-properties/data-nodes/data-node[data-node-name=0/RP0/CPU0]/system-view/interfaces/interface[interface-name=%s]", phys.Name)
}
