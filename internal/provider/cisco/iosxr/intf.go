// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

type PhysIf struct {
	Name         string        `json:"-"`
	Description  string        `json:"description,omitzero"`
	Statistics   Statistics    `json:"Cisco-IOS-XR-infra-statsd-cfg:statistics,omitzero"`
	MTUs         MTUs          `json:"mtus,omitzero"`
	Active       string        `json:"active,omitzero"`
	Vrf          string        `json:"Cisco-IOS-XR-infra-rsi-cfg:vrf,omitzero"`
	IPv4Network  IPv4Network   `json:"Cisco-IOS-XR-ipv4-io-cfg:ipv4-network,omitzero"`
	IPv6Network  IPv6Network   `json:"Cisco-IOS-XR-ipv6-ma-cfg:ipv6-network,omitzero"`
	IPv6Neighbor IPv6Neighbor  `json:"Cisco-IOS-XR-ipv6-nd-cfg:ipv6-neighbor,omitzero"`
	Shutdown     gnmiext.Empty `json:"shutdown,omitzero"`

	//BundleMember configuration for Physical interface as member of a Bundle-Ether
	BundleMember BundleMember `json:"Cisco-IOS-XR-bundlemgr-cfg:bundle-member,omitzero"`
}

type BundleMember struct {
	ID BundleID `json:"id"`
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

type BunldePortActivity string

const (
	PortActivityOn      BunldePortActivity = "on"
	PortActivityActive  BunldePortActivity = "active"
	PortActivityPassive BunldePortActivity = "passive"
	PortActivityInherit BunldePortActivity = "inherit"
)

// BundleInterface represents a port-channel (LAG) interface on IOS-XR devices
type BundleInterface struct {
	Name        string     `json:"-"`
	Description string     `json:"description,omitzero"`
	Statistics  Statistics `json:"Cisco-IOS-XR-infra-statsd-cfg:statistics,omitzero"`
	MTUs        MTUs       `json:"mtus,omitzero"`
	//mode in which an interface is running (e.g., virtual for subinterfaces)
	Mode gnmiext.Empty `json:"interface-virtual,omitzero"`

	//existence of this object causes the creation of the software subinterface
	ModeNoPhysical string           `json:"interface-mode-non-physical,omitzero"`
	Bundle         Bundle           `json:"Cisco-IOS-XR-bundlemgr-cfg:bundle,omitzero"`
	SubInterface   VlanSubInterface `json:"Cisco-IOS-XR-l2-eth-infra-cfg:vlan-sub-configuration,omitzero"`
}

type BundleID struct {
	BundleID    int32  `json:"bundle-id"`
	PortAcivity string `json:"port-activity"`
}

type Bundle struct {
	MinAct MinimumActive `json:"minimum-active"`
}

type MinimumActive struct {
	Links int32 `json:"links"`
}

type VlanSubInterface struct {
	VlanIdentifier VlanIdentifier `json:"vlan-identifier"`
}

type VlanIdentifier struct {
	FirstTag  int32  `json:"first-tag"`
	SecondTag int32  `json:"second-tag"`
	VlanType  string `json:"vlan-type"`
}

func (i *PhysIf) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration[active=act][interface-name=%s]", i.Name)
}

func (i *PhysIf) String() string {
	return fmt.Sprintf("Name: %s, Description=%s", i.Name, i.Description)
}

func (i *BundleInterface) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration[active=act][interface-name=%s]", i.Name)
}

func (i *BundleInterface) String() string {
	return fmt.Sprintf("Name: %s, Description=%s", i.Name, i.Description)
}

type IFaceSpeed string

const (
	Speed10G    IFaceSpeed = "TenGigE"
	Speed25G    IFaceSpeed = "TwentyFiveGigE"
	Speed40G    IFaceSpeed = "FortyGigE"
	Speed100G   IFaceSpeed = "HundredGigE"
	EtherBundle IFaceSpeed = "etherbundle"
)

func ExractMTUOwnerFromIfaceName(ifaceName string) (IFaceSpeed, error) {
	//MTU owner of bundle interfaces is 'etherbundle'
	bundleEtherRE := regexp.MustCompile(`^Bundle-Ether*`)
	if bundleEtherRE.MatchString(ifaceName) {
		// For Bundle-Ether interfaces
		return EtherBundle, nil
	}

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

func CheckInterfaceNameTypeAggregate(name string) error {
	if name == "" {
		return errors.New("interface name must not be empty")
	}
	//Matches Bundle-Ether<VLAN>[.<VLAN>] or BE<VLAN>[.<VLAN>]
	re := regexp.MustCompile(`^(Bundle-Ether|BE)(\d+)(\.(\d+))?$`)
	matches := re.FindStringSubmatch(name)

	if matches == nil {
		return fmt.Errorf("unsupported interface format %q, expected one of: %q", name, re.String())
	}

	//Vlan is part of the name
	if matches[2] == "" {
		return fmt.Errorf("unsupported interface format %q, expected one of: %q", name, re.String())
	}
	//Check outer vlan
	//fixme: check range up to 65000
	//err := CheckVlanRange(matches[2])

	//Check inner vlan if we have a subinterface
	if matches[4] != "" {
		return CheckVlanRange(matches[4])
	}
	return nil
}

func ExtractBundleIdAndVlanTagsFromName(name string) (int32, int32) {
	//Matches BE1.1 or Bundle-Ether1.1
	re := regexp.MustCompile(`^(Bundle-Ether|BE)(\d+)(?:\.(\d+))?$`)
	matches := re.FindStringSubmatch(name)

	bundleID := int32(0)
	outerVlan := int32(0)
	switch len(matches) {
	case 4:
		o, _ := strconv.Atoi(matches[2])
		bundleID = int32(o)
	case 5:
		o, _ := strconv.Atoi(matches[2])
		i, _ := strconv.Atoi(matches[3])
		bundleID = int32(o)
		outerVlan = int32(i)
	}
	return bundleID, outerVlan
}

func CheckVlanRange(vlan string) error {
	v, err := strconv.Atoi(vlan)

	if err != nil {
		return fmt.Errorf("failed to parse VLAN %q: %w", vlan, err)
	}

	if v < 1 || v > 4095 {
		return fmt.Errorf("VLAN %s is out of range, valid range is 1-4095", vlan)
	}
	return nil
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
