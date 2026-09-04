// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.UserProvider = (*Provider)(nil)

func (p *Provider) EnsureUser(ctx context.Context, req *provider.EnsureUserRequest) error {
	if len(req.Roles) > 1 {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.roles",
			Description: "the OpenConfig user model supports only a single role",
		})
	}
	u := &User{
		Username: req.Username,
		Config: &UserConfig{
			Username: req.Username,
			Role:     req.Roles[0],
			Password: req.Password,
			SSHKey:   req.SSHKey,
		},
	}
	return p.client.Patch(ctx, u)
}

func (p *Provider) DeleteUser(ctx context.Context, req *provider.DeleteUserRequest) error {
	return p.client.Delete(ctx, &User{Username: req.Username})
}

// Compile-time assertion.
var _ gnmiext.DataElement = (*User)(nil)

// User targets an OpenConfig user entry.
type User struct {
	Username string      `json:"-"`
	Config   *UserConfig `json:"config,omitempty"`
}

func (u *User) XPath() string {
	return fmt.Sprintf("openconfig-system:system/aaa/authentication/users/user[username=%s]", u.Username)
}

// UserConfig holds the user config container leaves.
// Password is write-only — the device returns a hashed value that would never match
// the plaintext, so we exclude it from unmarshal to avoid perpetual diffs.
type UserConfig struct {
	Username string `json:"username"`
	Role     string `json:"role,omitempty"`
	Password string `json:"password,omitempty"`
	SSHKey   string `json:"ssh-key,omitempty"`
}

func (c *UserConfig) UnmarshalJSON(data []byte) error {
	type alias struct {
		Username string `json:"username"`
		Role     string `json:"role,omitempty"`
		SSHKey   string `json:"ssh-key,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.Username = a.Username
	c.Role = a.Role
	c.SSHKey = a.SSHKey
	return nil
}
