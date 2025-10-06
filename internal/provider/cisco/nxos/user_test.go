// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "testing"

func TestEncoder(t *testing.T) {
	tests := []struct {
		name string
		enc  Encoder
		want string
	}{
		{
			name: "type5",
			enc:  Encrypt{Salt: []byte("OMJELK")},
			want: "$5$OMJELK$bZ.rikTqhYHhWZ5jw.MbtPpKivz8eSKnIHTLN9b4zXD",
		},
		{
			name: "pbkdf2/type8",
			enc:  PBKDF2{Salt: salt(t, "hhbDfWghBSrkpE")},
			want: "$nx-pbkdf2$hhbDfWghBSrkpE$6Whp10GyRx8J82oTUHJ.Z3WBV2TtmqISWXZ/pFFvDx2",
		},
		{
			name: "scrypt/type9",
			enc:  Scrypt{Salt: salt(t, "1hYvWojAYSY7nk")},
			want: "$nx-scrypt$1hYvWojAYSY7nk$fHkseAeAtlTv8az4j1/HuwOAVwWxK9bIrSTHy4wdOiU",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, err := test.enc.Encode("admin")
			if err != nil {
				t.Errorf("Encode() error = %v", err)
				return
			}
			if got != test.want {
				t.Errorf("Encode() = %v, want %v", got, test.want)
			}
		})
	}
}

// salt decodes a cisco base64-encoded salt string to a 10-byte array.
func salt(t *testing.T, saltstr string) [10]byte {
	t.Helper()
	salt, err := CiscoEncoding.DecodeString(saltstr)
	if err != nil {
		t.Fatal(err)
	}
	var s [10]byte
	copy(s[:], salt)
	return s
}

func init() {
	dom := &UserDomain{Name: "all"}
	dom.RoleItems.UserRoleList = append(dom.RoleItems.UserRoleList, &UserRole{Name: "network-admin"})
	user := &User{
		AllowExpired:   "no",
		Expiration:     "never",
		Name:           "johndoe",
		Pwd:            "Pa$$w0rd",
		PwdHash:        PwdHashTypePbkdf2,
		PwdEncryptType: PwdEncryptTypeClear,
	}
	user.SshauthItems.Data = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEgsAKZn/hxPMKyfwKboiOEeuL9bTqW79QfEQ8h0kpGhkFJJEWR1e3BvXpdT9KYQOaKQnNw32atULweSQQNGh6S73FvEIwYViuNCmygDxpiaJLIiYHAfs3NQ8wGG70l+DK6vPhkcO6uvq2XRP+y1W9gMAKlgMPj5BCl2LR6HUO9/Jzvi1yRX4w4E5shpvcVoUUB8ubFJ0IyfMTXb/sQrFvjq4ukH3wAV4CMrsP6fj5FoAQzJw3jlK5GCtK8FqkUkROBexwWGbFFjSbox5KXT2qludLocyQtw10rB6G/3af40tQJLHd0u6LnaCgHGfPod3Z9u2aL6DR1k5hBtGXGWxZ IronCore Test"
	user.UserdomainItems.UserDomainList = []*UserDomain{dom}
	Register("user", user)
}
