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

// DefaultLinkMTU is calculated based on the default MTU of 1500 bytes for routed interfaces on IOS-XR and the addition of 14 bytes for the L2 header.
// L2-payload + L2-header (1500 + 14)
const DefaultLinkMTU int32 = 1514

// DefaultL3MTU configures the maximum packet size of that protocol which includes the L3 header
// For subinterfaces automatically
// add 4 bytes for each VLAN tag configured on the sub-interface.
// add 8 bytes for a IEEE 802.1Q tunneling (QinQ) sub-interface.
const DefaultL3MTU int32 = 1500

var (
	bundleEtherRE       = regexp.MustCompile(`^(Bundle-Ether|bundle-ether)(\d+)(\.\d+)?$`)
	physicalInterfaceRE = regexp.MustCompile(`^(TenGigE|TwentyFiveGigE|FortyGigE|HundredGigE|GigabitEthernet)(\d){1}(\/\d){2}(\/\d+){1}(.\d{1,5})?$`)
	loopbackInterfaceRE = regexp.MustCompile(`^(Loopback|Lo)\d+$`)
	mgmtEthInterfaceRe  = regexp.MustCompile(`^MgmtEth\d+\/RP\d+\/CPU\d+\/\d+$`)
)

type IFaceOwner string

const (
	Speed10G    IFaceOwner = "TenGigE"
	Speed25G    IFaceOwner = "TwentyFiveGigE"
	Speed40G    IFaceOwner = "FortyGigE"
	Speed100G   IFaceOwner = "HundredGigE"
	EtherBundle IFaceOwner = "etherbundle"
	LoopBack    IFaceOwner = "Loopback"
	MgmtEth     IFaceOwner = "MgmtEth"
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
	PhysIfList []struct {
		Name string `json:"interface-name"`
	} `json:"interface-configuration"`
}

func (*Ifaces) XPath() string {
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

	// Required for subinterfaces
	// Existence of this object causes the creation of the software subinterface
	ModeNoPhysical string `json:"interface-mode-non-physical,omitzero"`

	// BundleMember configuration for Physical interface as member of a Bundle-Ether
	BundleMember BundleMember `json:"Cisco-IOS-XR-bundlemgr-cfg:bundle-member,omitzero"`

	// Required for bundle-interfaces
	// Existence of this object causes the creation of the interface within the bundlemgr database
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
	MTU       uint16        `json:"mtu,omitzero"`
}

type AddressesIPv4 struct {
	Primary Primary `json:"primary"`
}

type Primary struct {
	Address string `json:"address"`
	Netmask string `json:"netmask"`
}

type IPv6Network struct {
	MTU       uint16        `json:"mtu"`
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

// Extract the owner of an interface based on the interface name.
// For physical interfaces the owner matches the speed extracted from the interface name, e.g. 'TenGigE' for interface TwentyFiveGigE0/0/0/3
// For bundle interfaces the owner is 'etherbundle'
func ExtractOwnerFromInterfaceName(ifaceName string) (IFaceOwner, error) {
	if bundleEtherRE.MatchString(ifaceName) {
		return EtherBundle, nil
	}

	if loopbackInterfaceRE.MatchString(ifaceName) {
		return LoopBack, nil
	}

	if mgmtEthInterfaceRe.MatchString(ifaceName) {
		return MgmtEth, nil
	}

	// Match the port_type in an interface name <port_type>/<rack>/<slot/<module>/<port>
	// E.g. match TwentyFiveGigE of interface with name TwentyFiveGigE0/0/0/1
	re := regexp.MustCompile(`^\D*`)
	speed := string(re.Find([]byte(ifaceName)))
	if speed == "" {
		return "", fmt.Errorf("failed to extract interface speed from interface name %q", ifaceName)
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
		return "", fmt.Errorf("unsupported interface speed %q", speed)
	}
}

func MapInterfaceOwnerToSpeed(speed IFaceOwner) (int32, error) {
	switch speed {
	case Speed10G:
		return 10_000, nil
	case Speed25G:
		return 25_000, nil
	case Speed40G:
		return 40_000, nil
	case Speed100G:
		return 100_000, nil
	case EtherBundle, LoopBack, MgmtEth:
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported interface owner %s", speed)
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

// ExtractBundleAndSubinterfaceID extracts the subinterface ID and bundle ID from an interface name.
// If the interface is a physical interface, the bundle ID will be 0 and the subinterface ID will be extracted if present.
// TwentyFiveGigE0/0/0/3.4095 -> (0, 4095), Bundle-Ether200 -> (200, 0), Bundle-Ether200.4095 -> (200, 4095)
func ExtractBundleAndSubinterfaceID(name string) (int32, int32, error) {
	beMatchGroups := bundleEtherRE.FindStringSubmatch(name)
	physMatchGroups := physicalInterfaceRE.FindStringSubmatch(name)

	var bID string
	var sID string

	var bundleID int32
	var subIfaceID int32

	switch {
	case len(beMatchGroups) == 3:
		bID = beMatchGroups[2]
	case len(beMatchGroups) == 4:
		bID = beMatchGroups[2]
		sID = strings.ReplaceAll(beMatchGroups[3], ".", "")
	case len(physMatchGroups) == 6:
		sID = strings.ReplaceAll(physMatchGroups[5], ".", "")
	default:
		return 0, 0, fmt.Errorf("interface name %q does not start with Bundle-Ether or bundle-ether or match physical interface pattern", name)
	}

	if bID != "" {
		bundleIDInt, err := strconv.ParseInt(bID, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse bundle ID from interface name %q: %w", name, err)
		}
		bundleID = int32(bundleIDInt)
	}

	if sID != "" {
		subIfaceIDInt, err := strconv.ParseInt(sID, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse subinterface ID from interface name %q: %w", name, err)
		}
		subIfaceID = int32(subIfaceIDInt)
	}

	return bundleID, subIfaceID, nil
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
