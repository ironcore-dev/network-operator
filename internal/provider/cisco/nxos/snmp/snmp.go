// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package snmp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*SNMP)(nil)

type SNMP struct {
	// System Contact Information
	Contact string
	// System Location Information
	Location string
	// Source interface to be used for sending out SNMP notifications
	SrcIf string
	// The IPv4 ACL name to filter SNMP requests
	IPv4ACL string
	// Hosts to receive SNMP notifications
	Hosts []Host
	// Communities
	Communities []Community
	// Traps groups to enable.
	Traps []string
}

type Host struct {
	// IPv4 or IPv6 address or DNS Name of SNMP notification host
	Address string
	// SNMP community string or SNMPv3 user name (Max Size 32)
	Community string
	// SNMP version to use for the notification messages. Default 'v1'.
	Version Version
	// SNMP messages to sent to the host. Default 'traps'.
	Type MessageType
	// Configures SNMP to use the selected VRF to communicate with the host receiver
	Vrf string
}

type Community struct {
	// Community string
	Name string
	// Group to which the community belongs
	Group string
	// IPv4 ACL Name
	IPv4ACL string
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=Version
type Version uint8

const (
	V1 Version = iota
	V2c
	V3
)

func VersionFrom(s string) Version {
	switch strings.ToLower(s) {
	case "v1":
		return V1
	case "v2c":
		return V2c
	case "v3":
		return V3
	default:
		return V1
	}
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=MessageType
type MessageType uint8

const (
	Traps MessageType = iota
	Informs
)

func MessageTypeFrom(s string) MessageType {
	switch strings.ToLower(s) {
	case "traps":
		return Traps
	case "informs":
		return Informs
	default:
		return Traps
	}
}

// ToYGOT converts the SNMP configuration to YGOT updates for Cisco NX-OS devices.
// It configures various SNMP components with the following default values:
//
// Communities:
//   - Default group: "network-operator" (used when Community.Group is empty)
//   - Access level: unspecified (CommAcess = unspecified)
//
// Hosts:
//   - Default port: 162 (standard SNMP trap port)
//   - Default security level: noauth (for v1/v2c), auth (for v3)
//   - Default notification type: traps (when Host.Type is not specified)
//   - Default version: v1 (when Host.Version is not specified)
//
// Traps:
//   - All traps are initially disabled (EnableAllViaCLI = UNSET)
//   - Individual traps are enabled by setting Trapstatus = enable
//
// System Information:
//   - Empty strings are converted to "DME_UNSET_PROPERTY_MARKER" for deletion
func (s *SNMP) ToYGOT(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	var res nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems
	if err := client.Get(ctx, "System/snmp-items/inst-items/lclUser-items", &res); err != nil {
		return nil, err
	}
	communities := &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_CommunityItems{}
	for _, comm := range s.Communities {
		if c := communities.GetCommSecPList(comm.Name); c != nil {
			return nil, fmt.Errorf("snmp: duplicate snmp community %q", comm.Name)
		}
		c := communities.GetOrCreateCommSecPList(comm.Name)
		c.CommAcess = nxos.Cisco_NX_OSDevice_Snmp_CommAcessT_unspecified
		const group = "network-operator"
		c.GrpName = ygot.String(group)
		if comm.Group != "" {
			c.GrpName = ygot.String(comm.Group)
		}
		if comm.IPv4ACL != "" {
			c.GetOrCreateAclItems().UseIpv4AclName = ygot.String(comm.IPv4ACL)
		}
	}
	hosts := &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_HostItems{}
	for _, host := range s.Hosts {
		const port = 162
		if h := hosts.GetHostList(host.Address, port); h != nil {
			return nil, fmt.Errorf("snmp: duplicate snmp server host %q", host.Address)
		}
		h := hosts.GetOrCreateHostList(host.Address, port)
		h.CommName = ygot.String(host.Community)
		h.SecLevel = nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_noauth
		switch v := host.Type; v {
		case Traps:
			h.NotifType = nxos.Cisco_NX_OSDevice_Snmp_NotificationType_traps
		case Informs:
			h.NotifType = nxos.Cisco_NX_OSDevice_Snmp_NotificationType_informs
		default:
			return nil, fmt.Errorf("snmp: invalid message type %s", v)
		}
		switch v := host.Version; v {
		case V1:
			h.Version = nxos.Cisco_NX_OSDevice_Snmp_Version_v1
		case V2c:
			h.Version = nxos.Cisco_NX_OSDevice_Snmp_Version_v2c
		case V3:
			h.Version = nxos.Cisco_NX_OSDevice_Snmp_Version_v3
			h.SecLevel = nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_auth
		default:
			return nil, fmt.Errorf("snmp: invalid version %s", v)
		}
		if host.Vrf != "" {
			h.GetOrCreateUsevrfItems().GetOrCreateUseVrfList(host.Vrf)
		}
	}
	traps := &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems{}
	ygot.BuildEmptyTree(traps)
	traps.PopulateDefaults()
	traps.EnableAllViaCLI = nxos.Cisco_NX_OSDevice_Snmp_AllTrapsType_UNSET
	for _, t := range s.Traps {
		parts := strings.Fields(t)
		rv := reflect.ValueOf(traps).Elem()
		for len(parts) > 0 {
			name := strings.ToUpper(parts[0][:1]) + parts[0][1:]
			name = strings.TrimSuffix(name, "-items") + "Items"
			name = strings.ReplaceAll(name, "-", "")
			rv = rv.FieldByName(name)
			if !rv.IsValid() {
				return nil, fmt.Errorf("snmp: trap %q not found", t)
			}
			parts = parts[1:]
			rv = rv.Elem()
		}
		state := rv.FieldByName("Trapstatus")
		if !state.IsValid() {
			return nil, fmt.Errorf("feat: trap %q does not have Trapstatus", t)
		}
		admin := nxos.Cisco_NX_OSDevice_Snmp_SnmpTrapSt_enable
		if !state.Type().AssignableTo(reflect.TypeOf(admin)) {
			return nil, fmt.Errorf("feat: field 'Trapstatus' is not assignable to %T", admin)
		}
		if !state.CanSet() {
			return nil, errors.New("feat: field 'Trapstatus' cannot be set")
		}
		state.Set(reflect.ValueOf(admin))
	}
	updates := []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/snmp-items/inst-items/sysinfo-items",
			Value: &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_SysinfoItems{
				SysContact:  opt(s.Contact),
				SysLocation: opt(s.Location),
			},
		},
		gnmiext.EditingUpdate{
			XPath: "System/snmp-items/inst-items/globals-items/srcInterfaceTraps-items",
			Value: &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_GlobalsItems_SrcInterfaceTrapsItems{
				Ifname: opt(s.SrcIf),
			},
		},
		gnmiext.EditingUpdate{
			XPath: "System/snmp-items/inst-items/globals-items/srcInterfaceInforms-items",
			Value: &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_GlobalsItems_SrcInterfaceInformsItems{
				Ifname: opt(s.SrcIf),
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items/inst-items/community-items",
			Value: communities,
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items/inst-items/host-items",
			Value: hosts,
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items/inst-items/traps-items",
			Value: traps,
		},
	}

	for key := range res.LocalUserList {
		updates = append(updates, gnmiext.EditingUpdate{
			XPath: "System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=" + key + "]",
			Value: &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems_LocalUserList{
				Ipv4AclName: opt(s.IPv4ACL),
			},
			IgnorePaths: []string{
				"authpwd",
				"authtype",
				"group-items",
				"ipv6AclName",
				"isenforcepriv",
				"islocalizedV2key",
				"islocalizedkey",
				"privpwd",
				"privtype",
				"pwd_type",
				"userName",
				"usrengineId",
				"usrengineIdlen",
			},
		})
	}
	return updates, nil
}

func (s *SNMP) Reset(ctx context.Context, client gnmiext.Client) ([]gnmiext.Update, error) {
	var res nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_LclUserItems
	if err := client.Get(ctx, "System/snmp-items/inst-items/lclUser-items", &res); err != nil {
		return nil, err
	}
	updates := []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/sysinfo-items",
		},
		gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/globals-items/srcInterfaceTraps-items",
		},
		gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/globals-items/srcInterfaceInforms-items",
		},
		gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/host-items",
		},
		gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/community-items",
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items/inst-items/traps-items",
			Value: &nxos.Cisco_NX_OSDevice_System_SnmpItems_InstItems_TrapsItems{},
		},
	}
	for key := range res.LocalUserList {
		updates = append(updates, gnmiext.DeletingUpdate{
			XPath: "System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=" + key + "]/ipv4AclName",
		})
	}
	return updates, nil
}

func opt(s string) *string {
	if s == "" {
		return ygot.String("DME_UNSET_PROPERTY_MARKER")
	}
	return ygot.String(s)
}
