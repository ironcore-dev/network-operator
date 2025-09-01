// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"errors"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*User)(nil)

type User struct {
	Name     string
	Password string
	Encoder  Encoder
	SSHKey   string
	Roles    []Role
}

func NewUser(name, password string, opts ...UserOption) (*User, error) {
	if name == "" {
		return nil, errors.New("user name cannot be empty")
	}
	if password == "" {
		return nil, errors.New("user password cannot be empty")
	}
	u := &User{
		Name:     name,
		Password: password,
		Encoder:  Plain{},
	}
	for _, opt := range opts {
		if err := opt(u); err != nil {
			return nil, err
		}
	}
	return u, nil
}

type UserOption func(*User) error

// WithEncoder sets the Encoder for the user password.
func WithEncoder(encoder Encoder) UserOption {
	return func(u *User) error {
		if encoder == nil {
			return errors.New("encoder cannot be nil")
		}
		u.Encoder = encoder
		return nil
	}
}

// WithSSHKey sets the public SSH key the user can use for SSH authentication.
func WithSSHKey(publicKey string) UserOption {
	return func(u *User) error {
		if publicKey == "" {
			return errors.New("SSH key cannot be empty")
		}
		u.SSHKey = publicKey
		return nil
	}
}

// WithRoles sets the roles for the user.
func WithRoles(roles ...Role) UserOption {
	return func(u *User) error {
		if len(roles) == 0 {
			return errors.New("at least one role must be specified")
		}
		for _, r := range roles {
			if r.Name == "" {
				return errors.New("role name cannot be empty")
			}
		}
		u.Roles = roles
		return nil
	}
}

func (u *User) ToYGOT(client gnmiext.Client) ([]gnmiext.Update, error) {
	pwd, enc, err := u.Encoder.Encode(u.Password)
	if err != nil {
		return nil, err
	}
	inst := &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList{}
	inst.PopulateDefaults()
	inst.Name = ygot.String(u.Name)
	inst.Pwd = ygot.String(pwd)
	inst.PwdEncryptType = enc
	if u.SSHKey != "" {
		inst.SshauthItems = &nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList_SshauthItems{Data: ygot.String(u.SSHKey)}
	}
	for _, r := range u.Roles {
		inst.GetOrCreateUserdomainItems().GetOrCreateUserDomainList("all").GetOrCreateRoleItems().GetOrCreateUserRoleList(r.Name)
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/userext-items/user-items/User-list[name=" + u.Name + "]",
			Value: inst,
		},
	}, nil
}

func (u *User) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/userext-items/user-items/User-list[name=" + u.Name + "]",
		},
	}, nil
}

type Role struct {
	Name string
}

// Encoder is used to encode the user password before sending it to the device.
type Encoder interface {
	Encode(password string) (string, nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass, error)
}

var _ Encoder = Plain{}

type Plain struct{}

func (Plain) Encode(password string) (string, nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass, error) {
	return password, nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear, nil
}

// TODO(felix-kaestner): Add more encoders, e.g., Type5 Encrypt, PBKDF2, SCRYPT
