// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"errors"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		opts     []UserOption
		wantErr  bool
	}{
		{
			name:     "valid user",
			username: "testuser",
			password: "testpass",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			password: "testpass",
			wantErr:  true,
		},
		{
			name:     "empty password",
			username: "testuser",
			password: "",
			wantErr:  true,
		},
		{
			name:     "with encoder option",
			username: "testuser",
			password: "testpass",
			opts:     []UserOption{WithEncoder(Plain{})},
			wantErr:  false,
		},
		{
			name:     "with SSH key option",
			username: "testuser",
			password: "testpass",
			opts:     []UserOption{WithSSHKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...")},
			wantErr:  false,
		},
		{
			name:     "with roles option",
			username: "testuser",
			password: "testpass",
			opts:     []UserOption{WithRoles(Role{Name: "admin"})},
			wantErr:  false,
		},
		{
			name:     "with multiple options",
			username: "testuser",
			password: "testpass",
			opts: []UserOption{
				WithEncoder(Plain{}),
				WithSSHKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."),
				WithRoles(Role{Name: "admin"}, Role{Name: "network-operator"}),
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user, err := NewUser(test.username, test.password, test.opts...)
			if test.wantErr {
				if err == nil {
					t.Errorf("NewUser() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewUser() unexpected error = %v", err)
				return
			}
			if user == nil {
				t.Errorf("NewUser() returned nil")
				return
			}
			if user.Name != test.username {
				t.Errorf("NewUser() Name = %v, want %v", user.Name, test.username)
			}
			if user.Password != test.password {
				t.Errorf("NewUser() Password = %v, want %v", user.Password, test.password)
			}
			if user.Encoder == nil {
				t.Errorf("NewUser() Encoder is nil")
			}
		})
	}
}

func TestWithEncoder(t *testing.T) {
	tests := []struct {
		name    string
		encoder Encoder
		wantErr bool
	}{
		{
			name:    "valid encoder",
			encoder: Plain{},
			wantErr: false,
		},
		{
			name:    "nil encoder",
			encoder: nil,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := &User{}
			opt := WithEncoder(test.encoder)
			err := opt(user)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithEncoder() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithEncoder() unexpected error = %v", err)
				return
			}
			if user.Encoder != test.encoder {
				t.Errorf("WithEncoder() Encoder = %v, want %v", user.Encoder, test.encoder)
			}
		})
	}
}

func TestWithSSHKey(t *testing.T) {
	tests := []struct {
		name      string
		publicKey string
		wantErr   bool
	}{
		{
			name:      "valid SSH key",
			publicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
			wantErr:   false,
		},
		{
			name:      "empty SSH key",
			publicKey: "",
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := &User{}
			opt := WithSSHKey(test.publicKey)
			err := opt(user)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithSSHKey() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithSSHKey() unexpected error = %v", err)
				return
			}
			if user.SSHKey != test.publicKey {
				t.Errorf("WithSSHKey() SSHKey = %v, want %v", user.SSHKey, test.publicKey)
			}
		})
	}
}

func TestWithRoles(t *testing.T) {
	tests := []struct {
		name    string
		roles   []Role
		wantErr bool
	}{
		{
			name:    "valid single role",
			roles:   []Role{{Name: "admin"}},
			wantErr: false,
		},
		{
			name:    "valid multiple roles",
			roles:   []Role{{Name: "admin"}, {Name: "network-operator"}},
			wantErr: false,
		},
		{
			name:    "empty roles slice",
			roles:   []Role{},
			wantErr: true,
		},
		{
			name:    "role with empty name",
			roles:   []Role{{Name: ""}},
			wantErr: true,
		},
		{
			name:    "mixed valid and invalid roles",
			roles:   []Role{{Name: "admin"}, {Name: ""}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := &User{}
			opt := WithRoles(test.roles...)
			err := opt(user)
			if test.wantErr {
				if err == nil {
					t.Errorf("WithRoles() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("WithRoles() unexpected error = %v", err)
				return
			}
			if len(user.Roles) != len(test.roles) {
				t.Errorf("WithRoles() len(Roles) = %v, want %v", len(user.Roles), len(test.roles))
			}
			for i, role := range test.roles {
				if user.Roles[i].Name != role.Name {
					t.Errorf("WithRoles() Roles[%d].Name = %v, want %v", i, user.Roles[i].Name, role.Name)
				}
			}
		})
	}
}

func TestUser_ToYGOT(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		opts        []UserOption
		wantErr     bool
		wantXPath   string
		wantName    string
		wantPwd     string
		wantEncType nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass
		wantSSHKey  string
		wantRoles   []string
		checkSSHKey bool
		checkRoles  bool
	}{
		{
			name:        "basic user",
			username:    "testuser",
			password:    "testpass",
			wantErr:     false,
			wantXPath:   "System/userext-items/user-items/User-list[name=testuser]",
			wantName:    "testuser",
			wantPwd:     "testpass",
			wantEncType: nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear,
		},
		{
			name:        "user with SSH key",
			username:    "testuser",
			password:    "testpass",
			opts:        []UserOption{WithSSHKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...")},
			wantErr:     false,
			wantXPath:   "System/userext-items/user-items/User-list[name=testuser]",
			wantName:    "testuser",
			wantPwd:     "testpass",
			wantEncType: nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear,
			wantSSHKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
			checkSSHKey: true,
		},
		{
			name:        "user with roles",
			username:    "testuser",
			password:    "testpass",
			opts:        []UserOption{WithRoles(Role{Name: "admin"}, Role{Name: "network-operator"})},
			wantErr:     false,
			wantXPath:   "System/userext-items/user-items/User-list[name=testuser]",
			wantName:    "testuser",
			wantPwd:     "testpass",
			wantEncType: nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear,
			wantRoles:   []string{"admin", "network-operator"},
			checkRoles:  true,
		},
		{
			name:     "encoder error",
			username: "testuser",
			password: "testpass",
			opts:     []UserOption{WithEncoder(&mockEncoder{shouldError: true})},
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user, err := NewUser(test.username, test.password, test.opts...)
			if err != nil {
				t.Fatalf("NewUser() unexpected error = %v", err)
			}

			got, err := user.ToYGOT(t.Context(), &gnmiext.ClientMock{})
			if test.wantErr {
				if err == nil {
					t.Errorf("User.ToYGOT() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("User.ToYGOT() unexpected error = %v", err)
				return
			}

			if len(got) != 1 {
				t.Errorf("User.ToYGOT() expected 1 update, got %d", len(got))
				return
			}

			update, ok := got[0].(gnmiext.ReplacingUpdate)
			if !ok {
				t.Errorf("User.ToYGOT() expected update to be ReplacingUpdate")
				return
			}

			if update.XPath != test.wantXPath {
				t.Errorf("User.ToYGOT() expected XPath %v, got %v", test.wantXPath, update.XPath)
			}

			userList, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList)
			if !ok {
				t.Errorf("User.ToYGOT() expected value to be *nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems_UserList")
				return
			}

			if *userList.Name != test.wantName {
				t.Errorf("User.ToYGOT() expected Name %v, got %v", test.wantName, *userList.Name)
			}
			if *userList.Pwd != test.wantPwd {
				t.Errorf("User.ToYGOT() expected Pwd %v, got %v", test.wantPwd, *userList.Pwd)
			}
			if userList.PwdEncryptType != test.wantEncType {
				t.Errorf("User.ToYGOT() expected PwdEncryptType %v, got %v", test.wantEncType, userList.PwdEncryptType)
			}

			if test.checkSSHKey {
				if userList.SshauthItems == nil {
					t.Errorf("User.ToYGOT() expected SshauthItems to be configured")
					return
				}
				if *userList.SshauthItems.Data != test.wantSSHKey {
					t.Errorf("User.ToYGOT() expected SSH key %v, got %v", test.wantSSHKey, *userList.SshauthItems.Data)
				}
			}

			if test.checkRoles {
				if userList.UserdomainItems == nil {
					t.Errorf("User.ToYGOT() expected UserdomainItems to be configured")
					return
				}

				domainList := userList.UserdomainItems.GetUserDomainList("all")
				if domainList == nil {
					t.Errorf("User.ToYGOT() expected domain 'all' to be present")
					return
				}

				if domainList.RoleItems == nil {
					t.Errorf("User.ToYGOT() expected RoleItems to be configured")
					return
				}

				for _, roleName := range test.wantRoles {
					role := domainList.RoleItems.GetUserRoleList(roleName)
					if role == nil {
						t.Errorf("User.ToYGOT() expected role %v to be present", roleName)
					}
				}
			}
		})
	}
}

func TestUser_Reset(t *testing.T) {
	user, err := NewUser("testuser", "testpass")
	if err != nil {
		t.Fatalf("NewUser() unexpected error = %v", err)
	}

	got, err := user.Reset(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("User.Reset() unexpected error = %v", err)
		return
	}

	if len(got) != 1 {
		t.Errorf("User.Reset() expected 1 update, got %d", len(got))
		return
	}

	update, ok := got[0].(gnmiext.DeletingUpdate)
	if !ok {
		t.Errorf("User.Reset() expected update to be DeletingUpdate")
		return
	}

	expectedXPath := "System/userext-items/user-items/User-list[name=testuser]"
	if update.XPath != expectedXPath {
		t.Errorf("User.Reset() expected XPath %v, got %v", expectedXPath, update.XPath)
	}
}

func TestEncoder_Encode(t *testing.T) {
	tests := []struct {
		name        string
		encoder     Encoder
		password    string
		wantPwd     string
		wantEncType nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass
		wantErr     bool
	}{
		{
			name:        "Plain encoder",
			encoder:     Plain{},
			password:    "testpassword",
			wantPwd:     "testpassword",
			wantEncType: nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear,
			wantErr:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encodedPwd, encType, err := test.encoder.Encode(test.password)
			if test.wantErr {
				if err == nil {
					t.Errorf("Encoder.Encode() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Encoder.Encode() unexpected error = %v", err)
				return
			}

			if encodedPwd != test.wantPwd {
				t.Errorf("Encoder.Encode() encoded password = %v, want %v", encodedPwd, test.wantPwd)
			}
			if encType != test.wantEncType {
				t.Errorf("Encoder.Encode() encryption type = %v, want %v", encType, test.wantEncType)
			}
		})
	}
}

// mockEncoder is a test helper that implements the Encoder interface
type mockEncoder struct {
	shouldError bool
}

func (m *mockEncoder) Encode(password string) (string, nxos.E_Cisco_NX_OSDevice_Aaa_KeyEncUserPass, error) {
	if m.shouldError {
		return "", nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear, errors.New("mock encoder error")
	}
	return password, nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_clear, nil
}
