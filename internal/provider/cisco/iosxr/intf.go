// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

type IFaceSpeed string

const (
	Speed10G    IFaceSpeed = "TenGigE"
	Speed25G    IFaceSpeed = "TwentyFiveGigE"
	Speed40G    IFaceSpeed = "FortyGigE"
	Speed100G   IFaceSpeed = "HundredGigE"
	EtherBundle IFaceSpeed = "etherbundle"
)

type BundlePortActivity string

const (
	PortActivityOn      BundlePortActivity = "on"
	PortActivityActive  BundlePortActivity = "active"
	PortActivityPassive BundlePortActivity = "passive"
	PortActivityInherit BundlePortActivity = "inherit"
)

type PhysIfStateType string

const (
	StateUp        PhysIfStateType = "im-state-up"
	StateDown      PhysIfStateType = "im-state-down"
	StateNotReady  PhysIfStateType = "im-state-not-ready"
	StateAdminDown PhysIfStateType = "im-state-admin-down"
	StateShutDown  PhysIfStateType = "im-state-shutdown"
)

type Ifaces struct {
	PhysIfList []*Iface `json:"interface-configuration"`
}

func (i *Ifaces) XPath() string {
	return "Cisco-IOS-XR-ifmgr-cfg:interface-configurations"
}

// Iface represents physical and bundle interfaces as part of the same struct as they share a lot of common configuration
// and only differ in a few attributes like the interface name and the presence of bundle configuration or not.
type Iface struct {
	Name         string        `json:"interface-name"`
	Description  string        `json:"description,omitzero"`
	Statistics   Statistics    `json:"Cisco-IOS-XR-infra-statsd-cfg:statistics,omitzero"`
	MTUs         MTUs          `json:"mtus,omitzero"`
	Active       string        `json:"active,omitzero"`
	Vrf          string        `json:"Cisco-IOS-XR-infra-rsi-cfg:vrf,omitzero"`
	IPv4Network  IPv4Network   `json:"Cisco-IOS-XR-ipv4-io-cfg:ipv4-network,omitzero"`
	IPv6Network  IPv6Network   `json:"Cisco-IOS-XR-ipv6-ma-cfg:ipv6-network,omitzero"`
	IPv6Neighbor IPv6Neighbor  `json:"Cisco-IOS-XR-ipv6-nd-cfg:ipv6-neighbor,omitzero"`
	Shutdown     gnmiext.Empty `json:"shutdown,omitzero"`

	// Existence of this object causes the creation of the software subinterface
	ModeNoPhysical string `json:"interface-mode-non-physical,omitzero"`

	// BundleMember configuration for Physical interface as member of a Bundle-Ether
	BundleMember BundleMember `json:"Cisco-IOS-XR-bundlemgr-cfg:bundle-member,omitzero"`

	// Mode in which an interface is running (e.g., virtual for subinterfaces)
	Mode         gnmiext.Empty    `json:"interface-virtual,omitzero"`
	Bundle       Bundle           `json:"Cisco-IOS-XR-bundlemgr-cfg:bundle,omitzero"`
	SubInterface VlanSubInterface `json:"Cisco-IOS-XR-l2-eth-infra-cfg:vlan-sub-configuration,omitzero"`
}

type BundleMember struct {
	ID BundleID `json:"id"`
}

type Statistics struct {
	LoadInterval uint8 `json:"load-interval"`
}

type IPv4Network struct {
	Addresses AddressesIPv4 `json:"addresses"`
	Mtu       uint16        `json:"mtu,omitzero"`
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

type BundleID struct {
	BundleID     int32  `json:"bundle-id"`
	PortActivity string `json:"port-activity"`
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
	SecondTag int32  `json:"second-tag,omitzero"`
	VlanType  string `json:"vlan-type"`
}

func (i *Iface) XPath() string {
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration[active=act][interface-name=%s]", i.Name)
}

func (i *Iface) String() string {
	return fmt.Sprintf("Name: %s, Description=%s", i.Name, i.Description)
}

type PhysIfState struct {
	State string `json:"state"`
	Name  string `json:"-"`
}

func (phys *PhysIfState) XPath() string {
	// (fixme): hardcoded route processor for the moment
	return fmt.Sprintf("Cisco-IOS-XR-ifmgr-oper:interface-properties/data-nodes/data-node[data-node-name=0/RP0/CPU0]/system-view/interfaces/interface[interface-name=%s]", phys.Name)
}

func ExtractOwnerFromInterfaceName(ifaceName string) (IFaceSpeed, error) {
	// Owner of bundle interfaces is 'etherbundle'
	bundleEtherRE := regexp.MustCompile(`^Bundle-Ether*`)
	if bundleEtherRE.MatchString(ifaceName) {
		// For Bundle-Ether interfaces
		return EtherBundle, nil
	}
	return ExtractInterfaceSpeedFromName(ifaceName)
}

func ExtractInterfaceSpeedFromName(ifaceName string) (IFaceSpeed, error) {
	// Match the port_type in an interface name <port_type>/<rack>/<slot/<module>/<port>
	// E.g. match TwentyFiveGigE of interface with name TwentyFiveGigE0/0/0/1
	re := regexp.MustCompile(`^\D*`)
	speed := string(re.Find([]byte(ifaceName)))
	if speed == "" {
		return "", fmt.Errorf("failed to extract speed from interface name %s", ifaceName)
	}

	switch speed {
	case string(Speed10G):
		return Speed10G, nil
	case string(Speed25G):
		return Speed25G, nil
	case string(Speed40G):
		return Speed40G, nil
	case string(Speed100G):
		return Speed100G, nil
	default:
		return "", fmt.Errorf("unsupported interface type %s§§", speed)
	}
}

func MapInterfaceSpeedToNumeric(speed IFaceSpeed) (int32, error) {
	switch speed {
	case Speed10G:
		return 10000, nil
	case Speed25G:
		return 25000, nil
	case Speed40G:
		return 40000, nil
	case Speed100G:
		return 100000, nil
	default:
		return 0, fmt.Errorf("unsupported interface speed %s", speed)
	}
}

func CheckInterfaceNameTypeAggregate(name string) error {
	if name == "" {
		return errors.New("interface name must not be empty")
	}
	// Matches Bundle-Ether<VLAN>[.<VLAN>] or BE<VLAN>[.<VLAN>]
	re := regexp.MustCompile(`^(Bundle-Ether|BE)(\d+)(\.(\d+))?$`)
	matches := re.FindStringSubmatch(name)

	if matches == nil {
		return fmt.Errorf("unsupported interface format %q, expected one of: %q", name, re.String())
	}

	// Vlan is part of the name
	if matches[2] == "" {
		return fmt.Errorf("unsupported interface format %q, expected one of: %q", name, re.String())
	}
	// Check outer vlan
	// fixme: check range up to 65000
	// err := CheckVlanRange(matches[2])

	// Check inner vlan if we have a subinterface
	if matches[4] != "" {
		return CheckVlanRange(matches[4])
	}
	return nil
}

func ExtractVlanTagFromName(name string) (vlanID int32, err error) {
	// TF0/0/0/3.2021 or Te0/0/0/3.2021 or Hun0/0/0/3.2021
	res := strings.Split(name, ".")
	switch len(res) {
	case 1:
		return 0, nil
	case 2:
		vlan, err := strconv.ParseInt(res[1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to parse VLAN ID from interface name %q: %w", name, err)
		}
		return int32(vlan), nil
	default:
		return 0, fmt.Errorf("unexpected interface name format %q, expected <interface> or <interface>.<vlan>", name)
	}
}

func ExtractBundleIdAndVlanTagsFromName(name string) (bundleID, outerVlan int32, err error) {
	// Matches BE1.1 or Bundle-Ether1.1
	re := regexp.MustCompile(`^(Bundle-Ether|BE)(\d+)(?:\.(\d+))?$`)
	matches := re.FindStringSubmatch(name)

	switch len(matches) {
	case 3:
		o, err := strconv.ParseInt(matches[2], 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse bundle ID from interface name %q: %w", name, err)
		}
		bundleID = int32(o)
	case 4:
		o, err := strconv.ParseInt(matches[2], 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse bundle ID from interface name %q: %w", name, err)
		}
		i, err := strconv.ParseInt(matches[3], 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse outer VLAN from interface name %q: %w", name, err)
		}
		bundleID = int32(o)
		outerVlan = int32(i)
	}
	return bundleID, outerVlan, nil
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
