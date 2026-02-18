// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"

	"github.com/go-crypt/crypt/algorithm/shacrypt"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*User)(nil)

// User represents a local user on a NX-OS device.
type User struct {
	AllowExpired   string         `json:"allowExpired"`
	Expiration     string         `json:"expiration"`
	Name           string         `json:"name"`
	Pwd            string         `json:"pwd,omitempty"` // #nosec G117
	PwdHash        PwdHashType    `json:"passwordHash,omitempty"`
	PwdEncryptType PwdEncryptType `json:"pwdEncryptType,omitempty"`
	SshauthItems   struct {
		Data string `json:"data,omitempty"`
	} `json:"sshauth-items,omitzero"`
	UserdomainItems struct {
		UserDomainList gnmiext.List[string, *UserDomain] `json:"UserDomain-list,omitzero"`
	} `json:"userdomain-items,omitzero"`
}

func (*User) IsListItem() {}

func (u *User) XPath() string {
	return "System/userext-items/user-items/User-list[name=" + u.Name + "]"
}

func (u *User) SetPassword(password string, encoder Encoder) error {
	pwd, encType, err := encoder.Encode(password)
	if err != nil {
		return err
	}
	u.Pwd = pwd
	u.PwdHash = PwdHashTypeUnspecified
	u.PwdEncryptType = encType
	// When using plain text passwords, instruct the device to hash it as type 9 (Scrypt).
	if _, ok := encoder.(Plain); ok {
		u.PwdHash = PwdHashTypeScrypt
	}
	return nil
}

type UserDomain struct {
	Name      string `json:"name"`
	RoleItems struct {
		UserRoleList gnmiext.List[string, *UserRole] `json:"UserRole-list,omitzero"`
	} `json:"role-items,omitzero"`
}

func (d *UserDomain) Key() string { return d.Name }

type UserRole struct {
	Name string `json:"name"`
}

func (r *UserRole) Key() string { return r.Name }

type PwdEncryptType string

const (
	PwdEncryptTypeClear   PwdEncryptType = "clear"
	PwdEncryptTypeEncrypt PwdEncryptType = "Encrypt" // Type 5
	PwdEncryptTypePbkdf2  PwdEncryptType = "Pbkdf2"  // Type 8
	PwdEncryptTypeScrypt  PwdEncryptType = "scrypt"  // Type 9
)

type PwdHashType string

const (
	PwdHashTypeUnspecified PwdHashType = "unspecified"
	PwdHashTypePbkdf2      PwdHashType = "pbkdf2" // Type 8
	PwdHashTypeScrypt      PwdHashType = "scrypt" // Type 9
)

// Encoder is used to encode the user password before sending it to the device.
type Encoder interface {
	Encode(password string) (string, PwdEncryptType, error)
}

var (
	_ Encoder = Plain{}
	_ Encoder = Encrypt{} // Type 5
	_ Encoder = PBKDF2{}  // Type 8
	_ Encoder = Scrypt{}  // Type 9
)

type Plain struct{}

func (Plain) Encode(password string) (string, PwdEncryptType, error) {
	return password, PwdEncryptTypeClear, nil
}

type Encrypt struct{ Salt []byte }

func (e Encrypt) Encode(password string) (string, PwdEncryptType, error) {
	h, err := shacrypt.New(
		shacrypt.WithVariant(shacrypt.VariantSHA256),
		shacrypt.WithIterations(shacrypt.IterationsDefaultOmitted),
		shacrypt.WithSaltLength(4),
	)
	if err != nil {
		return "", "", err
	}
	d, err := h.HashWithSalt(password, e.Salt)
	if err != nil {
		return "", "", err
	}
	return d.Encode(), PwdEncryptTypeEncrypt, nil
}

type PBKDF2 struct{ Salt [10]byte }

func (p PBKDF2) Encode(password string) (string, PwdEncryptType, error) {
	k, err := pbkdf2.Key(sha256.New, password, p.Salt[:], 20000, 32)
	if err != nil {
		return "", "", err
	}
	pwd := "$nx-pbkdf2$" + CiscoEncoding.EncodeToString(p.Salt[:]) + "$" + CiscoEncoding.EncodeToString(k)
	return pwd, PwdEncryptTypePbkdf2, nil
}

type Scrypt struct{ Salt [10]byte }

func (s Scrypt) Encode(password string) (string, PwdEncryptType, error) {
	k, err := scrypt.Key([]byte(password), s.Salt[:], 16384, 8, 1, 32)
	if err != nil {
		return "", "", err
	}
	pwd := "$nx-scrypt$" + CiscoEncoding.EncodeToString(s.Salt[:]) + "$" + CiscoEncoding.EncodeToString(k)
	return pwd, PwdEncryptTypeScrypt, nil
}

// CiscoEncoding is the non-standard, not documented alphabet used by Cisco for their
// base64 encoding.
//
// Taken from: https://github.com/BrettVerney/ciscoPWDhasher/blob/master/CiscoPWDhasher/__init__.py#L8-L11
var CiscoEncoding = base64.NewEncoding("./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz").WithPadding(base64.NoPadding)

func ParsePasswordSalt(hash string) (salt [10]byte, err error) {
	// Expected hash format: $<algo>$<salt>$<hash>
	parts := strings.SplitN(hash, "$", 4)
	if len(parts) < 3 {
		return salt, errors.New("invalid password hash format")
	}
	s, err := CiscoEncoding.DecodeString(parts[2])
	if err != nil {
		return salt, fmt.Errorf("invalid salt encoding: %w", err)
	}
	if len(s) != 10 {
		return salt, fmt.Errorf("invalid salt length: got %d, want 10", len(s))
	}
	copy(salt[:], s)
	return
}
