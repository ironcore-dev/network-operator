// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// YANG has an empty type that is represented as a JSON array with a single null value. Whenever the key is present in YANG, it should evaluate to true.
type Empty bool

func (e *Empty) UnmarshalJSON(b []byte) error {
	var elements []any
	if err := json.Unmarshal(b, &elements); err != nil {
		return err
	}

	// Check if the data is a JSON array with a single null value
	if len(elements) == 1 && elements[0] == nil {
		*e = true
		return nil
	}
	*e = false
	return nil
}

func (e Empty) MarshalJSON() ([]byte, error) {
	// If e is true, marshal as a JSON array with a single null value
	if e {
		return json.Marshal([]any{nil})
	}
	return nil, nil
}

type PhisIf struct {
	Name         string       `json:"-"`
	Description  string       `json:"description,omitempty"`
	Active       string       `json:"active,omitempty"`
	Vrf          string       `json:"Cisco-IOS-XR-infra-rsi-cfg:vrf,omitempty"`
	Statistics   Statistics   `json:"Cisco-IOS-XR-infra-statsd-cfg:statistics,omitzero"`
	IPv4Network  IPv4Network  `json:"Cisco-IOS-XR-ipv4-io-cfg:ipv4-network,omitzero"`
	IPv6Network  IPv6Network  `json:"Cisco-IOS-XR-ipv6-ma-cfg:ipv6-network,omitzero"`
	IPv6Neighbor IPv6Neighbor `json:"Cisco-IOS-XR-ipv6-nd-cfg:ipv6-neighbor,omitzero"`
	MTUs         MTUs         `json:"mtus,omitzero"`
	Shutdown     Empty        `json:"shutdown,omitzero"`
}

type Statistics struct {
	LoadInterval uint8 `json:"load-interval,omitzero"`
}

type IPv4Network struct {
	Addresses AddressesIPv4 `json:"addresses,omitzero"`
	Mtu       uint16        `json:"mtu,omitzero"`
}

type AddressesIPv4 struct {
	Primary Primary `json:"primary,omitzero"`
}

type Primary struct {
	Address string `json:"address,omitempty"`
	Netmask string `json:"netmask,omitempty"`
}

type IPv6Network struct {
	Mtu       uint16        `json:"mtu,omitzero"`
	Addresses AddressesIPv6 `json:"addresses,omitzero"`
}

type AddressesIPv6 struct {
	RegularAddresses RegularAddresses `json:"regular-addresses,omitzero"`
}

type RegularAddresses struct {
	RegularAddress []RegularAddress `json:"regular-address,omitempty"`
}

type RegularAddress struct {
	Address      string `json:"address,omitempty"`
	PrefixLength uint8  `json:"prefix-length,omitzero"`
	Zone         string `json:"zone,omitempty"`
}

type IPv6Neighbor struct {
	RASuppress bool `json:"ra-suppress,omitempty"`
}

type MTUs struct {
	MTU []MTU `json:"mtu,omitempty"`
}

type MTU struct {
	MTU   int32  `json:"mtu,omitzero"`
	Owner string `json:"owner,omitempty"`
}

func (i *PhisIf) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration[active=act][interface-name=%s]", i.Name)
}

func NewIface(name string) *PhisIf {
	return &PhisIf{
		Name:        name,
		Statistics:  Statistics{},
		IPv4Network: IPv4Network{},
		IPv6Network: IPv6Network{},
		MTUs:        MTUs{},
	}
}

func (i *PhisIf) String() string {
	return fmt.Sprintf("Name: %s, Description=%s, ShutDown=%t", i.Name, i.Description, i.Shutdown)
}

type IFaceSpeed string

const (
	TenGig        IFaceSpeed = "TenGigE"
	TwentyFiveGig IFaceSpeed = "TwentyFiveGigE"
	FortyGig      IFaceSpeed = "FortyGigE"
	HundredGig    IFaceSpeed = "HundredGigE"
)

func ExractMTUOwnerFromIfaceName(ifaceName string) (IFaceSpeed, error) {
	re := regexp.MustCompile(`^\D*`)

	mtuOwner := string(re.Find([]byte(ifaceName)))

	if mtuOwner == "" {
		return "", fmt.Errorf("failed to extract MTU owner from interface name %s", ifaceName)
	}

	switch mtuOwner {
	case string(TenGig):
		return TenGig, nil
	case string(TwentyFiveGig):
		return TwentyFiveGig, nil
	case string(FortyGig):
		return FortyGig, nil
	case string(HundredGig):
		return HundredGig, nil
	default:
		return "", fmt.Errorf("unsupported interface type %s for MTU owner extraction", mtuOwner)
	}
}

type PhysIfStates int

const (
	StateUp PhysIfStates = iota
	StateDown
	StateNotReady
	StateAdminDown
	StateShutDown
)

var stateMapping = map[string]PhysIfStates{
	"im-state-not-ready": StateNotReady,
	"im-state-down":      StateDown,
	"im-state-up":        StateUp,
	"im-state-shutdown":  StateShutDown,
}

type PhysIfState struct {
	State string `json:"state,omitempty"`
	Name  string `json:"-,omitempty"`
}

func (phys *PhysIfState) XPath() string {
	// (fixme): hardcoded route processor for the moment
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-oper:interface-properties/data-nodes/data-node[data-node-name=0/RP0/CPU0]/system-view/interfaces/interface[interface-name=%s]", phys.Name)
}
