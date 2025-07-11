// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
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
	// Whether to enable/disable SNMP. Diabling SNMP will remove not remove any related configuration such as users, hosts, etc.
	Enable bool
	// The contact information for the SNMP server.
	// This corresponds to the CLI command `snmp-server contact <name>`.
	Contact string
	// The location information for the SNMP server.
	// This corresponds to the CLI command `snmp-server location <name>`.
	Location string
	// Source interface to be used for sending out SNMP Trap notifications.
	// This corresponds to the CLI command `snmp-server source-interface traps <name>`
	SrcIf string
	// Users who can access the SNMP engine.
	// This corresponds to the CLI command `snmp-server user name auth {md5 | sha | sha-256 | sha-512} passphrase [priv [aes-128] passphrase]`.
	Users []*User
	// Hosts who act as SNMP Notification Receivers.
	// This corresponds to the CLI command `snmp-server host <ip-address> {traps | informs} version {1 | 2c | 3} {community | username}`.
	Hosts []*Host
	// SNMP communities.
	// This corresponds to the CLI command `snmp-server community <name> {group <group>}`
	Communities []*Community
	// Traps groups to enable.
	// This corresponds to the CLI command `snmp-server traps enable <name>`.
	Traps []string
}

// user "admin" is reserved for the system and cannot be modified or deleted.
type User struct {
	// Name of the user.
	Name string
	// Authentication password.
	AuthPwd string
	// Authentication parameters for the user.
	AuthType AuthType
	// Privacy password for the user.
	PrivacyPwd string
	// Enables AES-128 bit encryption when using privacy password.
	Encrypt bool
	// IPv4 ACL Name.
	IPv4ACL string
	// Groups the user is assigned to.
	Groups []string
}

type Host struct {
	// IP address of hostname of target host.
	Address string
	// Type of message to send to host. Default is traps.
	Type string
	// SNMP version. Default is v2c.
	Version Version
	// SNMP community name.
	Community string
	// VRF to use to source traffic to source.
	Vrf string
	// Security level for SNMPv3.
	SecLevel Sec
}

type Community struct {
	// Name of the community.
	Name string
	// Group to which the community belongs.
	Group string
	// ACL name to filter snmp requests.
	ACL string
}

// resets the SNMP configuration to the default values. It uses the client to fetch the current configuration
// and copy protected values to the return value. The following paths are copied:
// - /System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=admin]
// - /System/snmp-items/servershutdown-items
// When resetting we also zero out all traps items that require features that are not enabled by default.
// The following items require the `feature-set fcoe` to be enabled:
// - /System/snmp-items/inst-items/traps-items/fcns-items
// - /System/snmp-items/inst-items/traps-items/rscn-items
// - /System/snmp-items/inst-items/traps-items/zone-items
func (s *SNMP) Reset(client gnmiext.Client) ([]gnmiext.Update, error) {
	it := &nxos.Cisco_NX_OSDevice_System_SnmpItems{}
	it.PopulateDefaults()
	err := s.addValuesFromRemote(client, it)
	if err != nil {
		return nil, fmt.Errorf("snmp: failed to add values from remote: %w", err)
	}

	it.InstItems.TrapsItems.FcnsItems = nil
	it.InstItems.TrapsItems.RscnItems = nil
	it.InstItems.TrapsItems.ZoneItems = nil

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items",
			Value: it,
		},
	}, nil
}

func (s *SNMP) addValuesFromRemote(client gnmiext.Client, dst *nxos.Cisco_NX_OSDevice_System_SnmpItems) error {
	src := &nxos.Cisco_NX_OSDevice_System_SnmpItems{}
	if client == nil {
		return errors.New("snmp: client is nil")
	}
	err := client.Get(context.Background(), "System/snmp-items", src)
	if err != nil {
		return fmt.Errorf("snmp: failed to get snmp-items from remote: %w", err)
	}
	// admin user: /inst-items/lclUser-items/LocalUser-list[userName=admin]
	adminUserSrc := src.GetInstItems().GetLclUserItems().GetLocalUserList("admin")
	if adminUserSrc == nil {
		return errors.New("snmp: cannot find admin user in remote configuration")
	}
	adminUserDst := dst.GetOrCreateInstItems().GetOrCreateLclUserItems().GetOrCreateLocalUserList("admin")
	err = ygot.MergeStructInto(adminUserDst, adminUserSrc, &ygot.MergeOverwriteExistingFields{})
	if err != nil {
		return fmt.Errorf("snmp: failed to merge admin user: %w", err)
	}
	// shutdown items: /System/snmp-items/servershutdown-items/
	shutdownSrc := src.GetServershutdownItems()
	if shutdownSrc != nil {
		err = ygot.MergeStructInto(dst.GetOrCreateServershutdownItems(), shutdownSrc, &ygot.MergeOverwriteExistingFields{})
		if err != nil {
			return fmt.Errorf("snmp: failed to merge servershutdown-items: %w", err)
		}
	}

	// the following items are special cases that require the `feature-set fcoe` to be enabled (not enabled by default).
	// If we send to the device non-nil values for these items, the device will reject the configuration.
	if src.InstItems != nil && src.InstItems.TrapsItems != nil && src.InstItems.TrapsItems.FcnsItems == nil {
		dst.InstItems.TrapsItems.FcnsItems = nil
	}
	if src.InstItems != nil && src.InstItems.TrapsItems != nil && src.InstItems.TrapsItems.RscnItems == nil {
		dst.InstItems.TrapsItems.RscnItems = nil
	}
	if src.InstItems != nil && src.InstItems.TrapsItems != nil && src.InstItems.TrapsItems.ZoneItems == nil {
		dst.InstItems.TrapsItems.ZoneItems = nil
	}
	return nil
}

func (s *SNMP) createAndPopulateSNMPItems() (*nxos.Cisco_NX_OSDevice_System_SnmpItems, error) {
	snmpItems := &nxos.Cisco_NX_OSDevice_System_SnmpItems{AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled}
	snmpItems.PopulateDefaults()
	instItems := snmpItems.GetOrCreateInstItems()
	userItems := instItems.GetOrCreateLclUserItems()

	// we next append each user to the list
	for _, u := range s.Users {
		userInList := userItems.GetOrCreateLocalUserList(u.Name)
		userInList.PopulateDefaults()
		// authentication password
		auth, err := u.AuthType.toAuthType()
		if err != nil {
			return nil, errors.New("snmp: invalid auth type")
		}
		userInList.Authtype = nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_UNSET
		if u.AuthPwd != "" && auth != nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_UNSET && auth != nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_no {
			userInList.Authtype = auth
			userInList.Authpwd = ygot.String(u.AuthPwd)
		}

		// privacy password
		userInList.Privtype = nxos.Cisco_NX_OSDevice_Snmp_PrivTypeT_UNSET
		if u.PrivacyPwd != "" {
			userInList.Privpwd = ygot.String(u.PrivacyPwd)
			userInList.Privtype = nxos.Cisco_NX_OSDevice_Snmp_PrivTypeT_aes128
			userInList.PwdType = ygot.Uint16(0)
		}

		if u.IPv4ACL != "" {
			userInList.Ipv4AclName = ygot.String(u.IPv4ACL)
		}

		groupItems := userInList.GetOrCreateGroupItems()
		for _, g := range u.Groups {
			groupItems.GetOrCreateUserGroupList(g)
		}
	}

	hostItems := instItems.GetOrCreateHostItems()
	for _, h := range s.Hosts {
		hostInList := hostItems.GetOrCreateHostList(h.Address, 162) // default port
		hostInList.CommName = ygot.String(h.Community)

		version, err := h.Version.toVersion()
		if err != nil {
			return nil, err
		}
		hostInList.Version = version

		secLevel, err := h.SecLevel.toSecLevel()
		if err != nil {
			return nil, err
		}
		hostInList.SecLevel = secLevel

		hostInList.NotifType = nxos.Cisco_NX_OSDevice_Snmp_NotificationType_traps

		if h.Vrf != "" {
			vrfList := hostInList.GetOrCreateUsevrfItems()
			vrfList.GetOrCreateUseVrfList(h.Vrf)
		}
	}

	// create communities
	communityItems := instItems.GetOrCreateCommunityItems()
	communityItems.PopulateDefaults()
	for _, c := range s.Communities {
		plist := communityItems.GetOrCreateCommSecPList(c.Name)
		plist.PopulateDefaults()
		plist.Name = ygot.String(c.Name)
		plist.GrpName = ygot.String(c.Group)
		if c.ACL != "" {
			acl := plist.GetOrCreateAclItems()
			acl.UseAclName = ygot.String(c.ACL)
		}
	}

	info := instItems.GetOrCreateSysinfoItems()
	info.PopulateDefaults()
	if s.Contact != "" {
		info.SysContact = ygot.String(s.Contact)
	}
	if s.Location != "" {
		info.SysLocation = ygot.String(s.Location)
	}

	// create traps
	traps := instItems.GetOrCreateTrapsItems()

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
	return snmpItems, nil
}

// returns a list of updates that can be applied to the device configuration.
// The following paths will we queried and copied onto the return value:
// - /System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=admin]
// - /System/snmp-items/servershutdown-items
func (s *SNMP) ToYGOT(client gnmiext.Client) ([]gnmiext.Update, error) {
	snmpItems, err := s.createAndPopulateSNMPItems()
	if err != nil {
		return nil, err
	}
	if s.Enable {
		snmpItems.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	} else {
		snmpItems.AdminSt = nxos.Cisco_NX_OSDevice_Nw_AdminSt_disabled
	}
	// copy non-modifiable values from remote
	err = s.addValuesFromRemote(client, snmpItems)
	if err != nil {
		return nil, fmt.Errorf("snmp: failed to add values in remote config to new config: %w", err)
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/snmp-items",
			Value: snmpItems,
		},
	}, nil
}

//go:generate go tool stringer -type=AuthType

type AuthType uint8

const (
	MD5 AuthType = iota
	SHA
	SHA256
	SHA512
)

func (a AuthType) toAuthType() (nxos.E_Cisco_NX_OSDevice_Snmp_AuthTypeT, error) {
	switch a {
	case MD5:
		return nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_md5, nil
	case SHA:
		return nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_sha, nil
	case SHA256:
		return nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_sha_256, nil
	case SHA512:
		return nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_sha_512, nil
	default:
		return nxos.Cisco_NX_OSDevice_Snmp_AuthTypeT_UNSET, fmt.Errorf("snmp: invalid auth type %v", a)
	}
}

//go:generate go tool stringer -type=Version

type Version uint8

const (
	V1 Version = iota
	V2c
	V3
)

func (v Version) toVersion() (nxos.E_Cisco_NX_OSDevice_Snmp_Version, error) {
	switch v {
	case V1:
		return nxos.Cisco_NX_OSDevice_Snmp_Version_v1, nil
	case V2c:
		return nxos.Cisco_NX_OSDevice_Snmp_Version_v2c, nil
	case V3:
		return nxos.Cisco_NX_OSDevice_Snmp_Version_v3, nil
	default:
		return nxos.Cisco_NX_OSDevice_Snmp_Version_UNSET, fmt.Errorf("snmp: invalid version %v", v)
	}
}

//go:generate go tool stringer -type=Sec

type Sec uint8

const (
	NoAuth Sec = iota
	Auth
	Priv
)

func (s Sec) toSecLevel() (nxos.E_Cisco_NX_OSDevice_Snmp_V3SecLvl, error) {
	switch s {
	case NoAuth:
		return nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_noauth, nil
	case Auth:
		return nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_auth, nil
	case Priv:
		return nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_priv, nil
	default:
		return nxos.Cisco_NX_OSDevice_Snmp_V3SecLvl_UNSET, fmt.Errorf("snmp: invalid sec level %v", s)
	}
}
