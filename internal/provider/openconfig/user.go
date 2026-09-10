// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-crypt/crypt/algorithm/shacrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	hashedPassword, err := p.hashPassword(ctx, req.Username, req.Password)
	if err != nil {
		return fmt.Errorf("hashing password for user %q: %w", req.Username, err)
	}

	u := &User{
		Username: req.Username,
		Config: &UserConfig{
			Username:       req.Username,
			Role:           req.Roles[0],
			PasswordHashed: hashedPassword,
			SSHKey:         req.SSHKey,
		},
	}
	return p.client.Patch(ctx, u)
}

// hashPassword hashes the plaintext password using SHA-512 crypt ($6$).
// If the device already stores a hash for this user, its salt is reused so
// that the resulting hash is identical on every reconcile when the password
// has not changed (idempotency).
func (p *Provider) hashPassword(ctx context.Context, username, password string) (string, error) {
	current := &User{Username: username}
	switch err := p.client.GetConfig(ctx, current); {
	case err == nil && current.Config != nil:
		currentDigest, err := shacrypt.Decode(current.Config.PasswordHashed)
		if err != nil {
			return "", err
		}
		if currentDigest.Match(password) {
			return current.Config.PasswordHashed, nil
		}
	case err == nil, errors.Is(err, gnmiext.ErrNil), status.Code(err) == codes.NotFound:
		// user does not exist yet or config is empty, generate a fresh hash
	default:
		return "", err
	}

	hash, err := shacrypt.NewSHA512()
	if err != nil {
		return "", err
	}

	digest, err := hash.Hash(password)
	if err != nil {
		return "", err
	}
	return digest.Encode(), nil
}

func (p *Provider) DeleteUser(ctx context.Context, req *provider.DeleteUserRequest) error {
	return p.client.Delete(ctx, &User{Username: req.Username})
}

// Compile-time assertion.
var _ gnmiext.DataElement = (*User)(nil)

// User targets an OpenConfig user entry.
type User struct {
	Username string      `json:"-"`
	Config   *UserConfig `json:"config"`
}

func (u *User) XPath() string {
	return fmt.Sprintf("openconfig-system:system/aaa/authentication/users/user[username=%s]", u.Username)
}

// UserConfig holds the user config container leaves.
// PasswordHashed uses the password-hashed field which is stored by the
// device, enabling idempotent reconciliation.
type UserConfig struct {
	Username       string `json:"username"`
	Role           string `json:"role,omitempty"`
	PasswordHashed string `json:"password-hashed"`
	SSHKey         string `json:"ssh-key,omitempty"`
}
