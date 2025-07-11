// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package localusr

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func TestToYGOT(t *testing.T) {
	ignorePaths := []string{
		"/User-list[name=admin1]",
		"/User-list[name=testadmin1]",
	}
	users := &Users{
		UserList: []*User{
			{
				Name:           "testadmin1",
				Pwd:            "hashedtestpassword",
				PwdEncryptType: PasswdType5,
				Shelltype:      Bash,
				UserdomainItems: []UserDomain{
					{
						Name: "all",
						RoleItems: []*UserRole{
							{
								Name:     "network-admin",
								PrivType: NoDataPriv,
							},
						},
					},
				},
			},
			{
				Name:           "testadmin2",
				Pwd:            "hashedtestpassword",
				PwdEncryptType: PasswdType5,
				Shelltype:      VSH,
				UserdomainItems: []UserDomain{
					{
						Name: "all",
						RoleItems: []*UserRole{
							{
								Name:     "network-admin",
								PrivType: NoDataPriv,
							},
						},
					},
				},
			},
			{
				Name:           "testadmin3",
				Pwd:            "hashedtestpassword",
				PwdEncryptType: PasswdTypeScrypt,
				Shelltype:      Bash,
				UserdomainItems: []UserDomain{
					{
						Name: "domain1",
						RoleItems: []*UserRole{
							{
								Name:     "network-operator",
								PrivType: ReadPriv,
							},
						},
					},
					{
						Name: "domain2",
						RoleItems: []*UserRole{
							{
								Name:     "network-admin",
								PrivType: NoDataPriv,
							},
						},
					},
				},
			},
			{
				Name:           "testoperator1",
				Pwd:            "hashedtestpassword",
				PwdEncryptType: PasswdType5,
				Shelltype:      Bash,
			},
		},
		IgnorePaths: ignorePaths,
	}

	got, err := users.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("ToYGOT() error = %v", err)
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	// Validate the XPath of the first update
	if update.XPath != "System/userext-items/user-items" {
		t.Errorf("expected XPath 'System/userext-items/user-items', got %s", update.XPath)
	}

	// Validate the example UserList
	ui, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_UserextItems_UserItems")
	}
	if ui.UserList == nil {
		t.Fatalf("expected user list to be present")
	}

	// Validate the number of users in the list
	if len(ui.UserList) != 4 {
		t.Fatalf("expected 4 users in the list, got %d", len(ui.UserList))
	}

	// Validate the first user's name
	if _, ok := ui.UserList["testadmin1"]; !ok {
		t.Errorf("expected user 'testadmin1' to be present in the user list")
	}
	// Validate the shell type for user testadmin1
	if ui.UserList["testadmin1"].Shelltype != nxos.Cisco_NX_OSDevice_AaaLoginShellType_shellbash {
		t.Errorf("expected shell type 'shellbash' for user 'testadmin1', got %v", ui.UserList["testadmin1"].Shelltype)
	}
	// Validate the password encryption type for user testadmin1
	if ui.UserList["testadmin1"].PwdEncryptType != nxos.Cisco_NX_OSDevice_Aaa_KeyEncUserPass_Encrypt {
		t.Errorf("expected password encryption type 'Encrypt' for user 'testadmin1', got %v", ui.UserList["testadmin1"].PwdEncryptType)
	}
	// Validate the user domain items for user testadmin1
	if len(ui.UserList["testadmin1"].UserdomainItems.UserDomainList) != 1 {
		t.Errorf("expected 1 user domain item for user 'testadmin1', got %d", len(ui.UserList["testadmin1"].UserdomainItems.UserDomainList))
	}
	// Validate all user domain names for user testadmin3
	expectedDomains := []string{"domain1", "domain2"}
	actualDomains := ui.UserList["testadmin3"].UserdomainItems.UserDomainList

	if len(actualDomains) != len(expectedDomains) {
		t.Errorf("expected %d user domains for user 'testadmin3', got %d", len(expectedDomains), len(actualDomains))
	}

	for _, expectedDomain := range expectedDomains {
		if _, ok := actualDomains[expectedDomain]; !ok {
			t.Errorf("expected user domain '%s' for user 'testadmin3' to be present, but it is missing", expectedDomain)
		}
	}
	// Validate the PrivType assigned to testadmin2's role
	testadmin2Roles := ui.UserList["testadmin2"].UserdomainItems.UserDomainList["all"].RoleItems.UserRoleList
	if len(testadmin2Roles) != 1 {
		t.Errorf("expected 1 role for user 'testadmin2' in domain 'all', got %d", len(testadmin2Roles))
	}
	if testadmin2Roles["network-admin"].PrivType != nxos.Cisco_NX_OSDevice_Aaa_UserRolePrivType_noDataPriv {
		t.Errorf("expected PrivType 'noDataPriv' for role 'network-admin' of user 'testadmin2', got %v", testadmin2Roles["network-admin"].PrivType)
	}
	// Validate the ignore paths
	if len(users.IgnorePaths) != 2 {
		t.Fatalf("expected 2 ignore paths, got %d", len(users.IgnorePaths))
	}
	if users.IgnorePaths[0] != "/User-list[name=admin1]" {
		t.Errorf("expected ignore path '/User-list[name=admin1]', got %s", users.IgnorePaths[0])
	}
	if users.IgnorePaths[1] != "/User-list[name=testadmin1]" {
		t.Errorf("expected ignore path '/User-list[name=testadmin1]', got %s", users.IgnorePaths[1])
	}
}
