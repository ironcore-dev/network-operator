// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.UserProvider = (*Provider)(nil)

func (p *Provider) EnsureUser(ctx context.Context, req *provider.EnsureUserRequest) error {
	validErr := validateUserRequest(req)
	if validErr != nil && !isIgnoredFieldError(validErr) {
		return validErr
	}
	u := &User{
		Username: req.Username,
		Config: &UserConfig{
			Username: req.Username,
			Role:     req.Roles[0],
		},
	}
	if err := p.client.Update(ctx, u); err != nil {
		return err
	}
	if validErr != nil {
		log.FromContext(ctx).Info("User configured with ignored fields", "warning", validErr.Error())
	}
	return validErr
}

func (p *Provider) DeleteUser(ctx context.Context, req *provider.DeleteUserRequest) error {
	return p.client.Delete(ctx, &User{Username: req.Username})
}

func validateUserRequest(req *provider.EnsureUserRequest) error {
	var violations []apistatus.FieldViolation
	if req.SSHKey != "" {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.sshPublicKey",
			Description: "sshPublicKey is not supported by the OpenConfig user model on SRLinux",
		})
	}
	if len(req.Roles) > 1 {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.roles",
			Description: "only one role is supported by the OpenConfig user model on SRLinux; role name must be an OpenConfig AAA identity (e.g. openconfig-aaa-types:SYSTEM_ROLE_ADMIN)",
		})
	}
	if len(violations) > 0 {
		return apistatus.NewUnsupportedFieldError(violations...)
	}
	if req.Password != "" {
		return apistatus.NewIgnoredFieldError(apistatus.FieldViolation{
			Field:       "spec.password",
			Description: "password is not supported by the OpenConfig user model on SRLinux",
		})
	}
	return nil
}

func isIgnoredFieldError(err error) bool {
	se, ok := apistatus.FromError(err)
	return ok && se.Code == apistatus.CodeIgnoredField
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
// Role must be a valid OpenConfig AAA identity string, e.g. "openconfig-aaa-types:SYSTEM_ROLE_ADMIN".
// The device enforces this via a leafref — native role names (e.g. "admin") are rejected.
type UserConfig struct {
	Username string `json:"username"`
	Role     string `json:"role,omitempty"`
}
