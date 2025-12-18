// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	bundleEtherRE       = regexp.MustCompile(`^Bundle-Ether(\d+)(?:\.\d+)?$`)
	physicalInterfaceRE = regexp.MustCompile(`^(TenGigE|TwentyFiveGigE|FortyGigE|HundredGigE|GigabitEthernet)(\d){1}(\/\d){2}(\/\d+){1}(.\d{1,5})?$`)
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

// Iface represents physical and bundle interfaces as part of the same struct as they share a lot of common configuration
// and only differ in a few attributes like the interface name and the presence of bundle configuration or not.
type Iface struct {
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

func ValidateInterfaceName(name string) error {
	// Supported Interface name formats:
	// Physical Interface <PortSpeed><rack><slot><port> e.g TwentyFiveGigE0/0/0/3
	// SubInterface <PotySpeed><rack><slot><port>.<vlan-id> e.g TwentyFiveGigE0/0/0/3
	// Bundle Interface/Port Channel Bundle-Ether<BundleID>
	// Vlans over Bundle Bundle-Ether<BundleID>.<vlan-id>

	beErr := CheckInterfaceNameTypeAggregate(name)
	physErr := CheckInterfaceNameTypePhysical(name)

	if beErr == nil || physErr == nil {
		return nil
	}
	return fmt.Errorf("unsupported interface name format: %s", name)
}

func ExtractInterfaceSpeedFromName(ifaceName string) (IFaceSpeed, error) {
	// Owner of bundle interfaces is 'etherbundle'
	if bundleEtherRE.MatchString(ifaceName) {
		return EtherBundle, nil
	}

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
		return "", fmt.Errorf("unsupported interface type %s", speed)
	}
}

func CheckInterfaceNameTypeAggregate(name string) error {
	matches := bundleEtherRE.FindStringSubmatch(name)

	if matches == nil {
		return fmt.Errorf("unsupported interface format %q, expected one of: %q", name, bundleEtherRE.String())
	}
	// Fixme(sven-rosenzweig): check BundleId range
	return nil
}

func CheckInterfaceNameTypePhysical(name string) error {
	if !physicalInterfaceRE.MatchString(name) {
		return fmt.Errorf("unsupported physical interface format %s", name)
	}
	return nil
}

func ExtractVlanTagFromName(name string) (vlanID int32, err error) {
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

func ExtractBundleAndSubinterfaceID(name string) (bundleID, subinterfaceID int32, err error) {
	// Extract bundle ID and optional subinterface ID from Bundle-Ether<id> or Bundle-Ether<id>.<subif_id>
	// Examples: Bundle-Ether200 -> (200, 0), Bundle-Ether200.4095 -> (200, 4095)

	// Remove the "Bundle-Ether" or "BE" prefix
	var idPart string

	if !bundleEtherRE.MatchString(name) {
		return 0, 0, fmt.Errorf("interface name %q does not start with Bundle-Ether or bundle-ether", name)
	}
	idPart = strings.TrimPrefix(strings.TrimPrefix(name, "Bundle-Ether"), "bundle-ether")
	parts := strings.Split(idPart, ".")

	if len(parts) == 0 || parts[0] == "" {
		return 0, 0, fmt.Errorf("failed to extract bundle ID from interface name %q", name)
	}

	// Parse bundle ID
	id, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse bundle ID from interface name %q: %w", name, err)
	}
	bundleID = int32(id)

	// Parse subinterface ID if present
	if len(parts) == 2 {
		subIfaceIDInt, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse subinterface ID from interface name %q: %w", name, err)
		}
		subinterfaceID = int32(subIfaceIDInt)
	} else if len(parts) > 2 {
		return 0, 0, fmt.Errorf("unexpected interface name format %q, expected Bundle-Ether<id> or Bundle-Ether<id>.<subif_id>", name)
	}

	return bundleID, subinterfaceID, nil
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
