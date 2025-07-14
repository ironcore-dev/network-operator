// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// This package enables the configuration of local user accounts on the device as per the following configuration sample:
// - username testadmin2 password 5 <password hash> role network-operator
// - username testadmin2 role network-admin
// - username testadmin2 passphrase  lifetime 99999 warntime 14 gracetime 3
//
// Which corresponds to the following YANG model:
//   - System/userext-items/user-items/User-list[name=testadmin2]
//
// Implementation notes:
//   - The local users are configured using the user-list. The key of each item in the user-list is the 'name' attribute. Each user is defined with a name, password, shelltype, and roles.
//   - The role is a nested attribute of each defined user.  It is a list of roles assigned to the user.
//   - A default role will be associated with the user at account creation (network-operator) for read-only access.  If admin privileges are required, the user must be defined with the network-admin role.
//   - The user will be created with password expiration settings based on the default values defined on the device, this will be reflected in the username config line showing warntime, lifetime and gracetime.  By default, new accounts have no password expiration.
package localusr

import (
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*Users)(nil)

type UserRole struct {
	Name     string
	PrivType PrivType
}

type UserDomain struct {
	Name      string
	RoleItems []*UserRole
}

type UserDomainItems struct {
	UserDomainList []*UserDomain
}

type User struct {
	Name            string
	Pwd             string
	PwdEncryptType  PasswordType
	Shelltype       ShellType
	UserdomainItems []UserDomain
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=PasswordType
type PasswordType int

const (
	PasswdType5 PasswordType = iota + 1
	PasswdTypePbkdf2
	PasswdTypeScrypt
)

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=ShellType
type ShellType int

const (
	VSH ShellType = iota + 1
	Bash
)

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=PrivType
type PrivType int64

const (
	NoDataPriv PrivType = iota
	ReadPriv
	WritePriv
)

type Users struct {
	UserList    []*User
	IgnorePaths []string
}

func (u *User) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	users := &Users{
		UserList: []*User{u},
	}
	return users.ToYGOT(nil)
}

func (users *Users) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	userList := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems{UserList: make(map[string]*nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList, len(users.UserList))}

	for _, u := range users.UserList {
		userdomains := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems{
			UserDomainList: make(map[string]*nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems_UserDomainList, len(u.UserdomainItems)),
		}

		for _, dom := range u.UserdomainItems {
			userdom := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems_UserDomainList{
				Name: ygot.String(dom.Name),
				RoleItems: &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems_UserDomainList_RoleItems{
					UserRoleList: make(map[string]*nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems_UserDomainList_RoleItems_UserRoleList, len(dom.RoleItems)),
				},
			}

			for _, role := range dom.RoleItems {
				var privtype nxos.E_Cisco_NX_OSDevice_Aaa_UserRolePrivType
				switch role.PrivType {
				case NoDataPriv:
					privtype = nxos.Cisco_NX_OSDevice_Aaa_UserRolePrivType_noDataPriv
				case ReadPriv:
					privtype = nxos.Cisco_NX_OSDevice_Aaa_UserRolePrivType_readPriv
				case WritePriv:
					privtype = nxos.Cisco_NX_OSDevice_Aaa_UserRolePrivType_writePriv
				default:
					return nil, fmt.Errorf("localusr: invalid privilege type %d", role.PrivType)
				}

				roleitem := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_UserdomainItems_UserDomainList_RoleItems_UserRoleList{
					Name:     ygot.String(role.Name),
					PrivType: privtype,
				}
				err := userdom.RoleItems.AppendUserRoleList(roleitem)
				if err != nil {
					return nil, err
				}
			}

			err := userdomains.AppendUserDomainList(userdom)
			if err != nil {
				return nil, err
			}
		}

		var pwdEncType nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass
		switch u.PwdEncryptType {
		case PasswdType5:
			pwdEncType = nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_Encrypt
		case PasswdTypePbkdf2:
			pwdEncType = nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_Pbkdf2
		case PasswdTypeScrypt:
			pwdEncType = nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_scrypt
		default:
			return nil, fmt.Errorf("localusr: invalid password encrypt type %d", u.PwdEncryptType)
		}

		var shelltype nxos.E_Cisco_NX_OSDevice_AaaLoginShellType
		switch u.Shelltype {
		case VSH:
			shelltype = nxos.Cisco_NX_OSDevice_AaaLoginShellType_shellvsh
		case Bash:
			shelltype = nxos.Cisco_NX_OSDevice_AaaLoginShellType_shellbash
		default:
			return nil, fmt.Errorf("localusr: invalid shell type %d", u.Shelltype)
		}

		user := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList{
			Name:            ygot.String(u.Name),
			Pwd:             ygot.String(u.Pwd),
			PwdEncryptType:  pwdEncType,
			Shelltype:       shelltype,
			UserdomainItems: userdomains,
		}

		err := userList.AppendUserList(user)
		if err != nil {
			return nil, err
		}
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath:       "System/userext-items/user-items",
			Value:       userList,
			IgnorePaths: users.IgnorePaths,
		},
	}, nil
}

func (v *Users) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/userext-items/user-items",
			Value: &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems{},
		},
	}, nil
}
