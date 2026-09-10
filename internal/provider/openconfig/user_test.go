// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"errors"
	"testing"

	"github.com/go-crypt/crypt/algorithm/shacrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func newProviderWithClient(client gnmiext.Client) *Provider {
	return &Provider{client: client}
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := shacrypt.NewSHA512()
	if err != nil {
		t.Fatalf("shacrypt.NewSHA512: %v", err)
	}
	d, err := h.Hash(password)
	if err != nil {
		t.Fatalf("shacrypt hash: %v", err)
	}
	return d.Encode()
}

func TestHashPassword(t *testing.T) {
	const (
		username = "testuser"
		password = "secret"
	)

	existingHash := mustHashPassword(t, password)
	getConfigErr := errors.New("get config error")

	tests := []struct {
		name          string
		getConfigFunc func(ctx context.Context, elements ...gnmiext.DataElement) error
		wantHash      string // if set, exact hash expected
		wantMatch     bool   // if true, result must verify against password
		wantErr       bool
	}{
		{
			name: "no user in config (ErrNil) — fresh hash",
			getConfigFunc: func(_ context.Context, _ ...gnmiext.DataElement) error {
				return gnmiext.ErrNil
			},
			wantMatch: true,
		},
		{
			name: "no user in config (NotFound) — fresh hash",
			getConfigFunc: func(_ context.Context, _ ...gnmiext.DataElement) error {
				return status.Error(codes.NotFound, "not found")
			},
			wantMatch: true,
		},
		{
			name: "user exists with nil Config — fresh hash",
			getConfigFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
				// leave Config nil, return no error
				return nil
			},
			wantMatch: true,
		},
		{
			name: "password matches stored hash — returns existing hash unchanged",
			getConfigFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
				elements[0].(*User).Config = &UserConfig{PasswordHashed: existingHash}
				return nil
			},
			wantHash: existingHash,
		},
		{
			name: "password not matched — fresh hash generated",
			getConfigFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
				elements[0].(*User).Config = &UserConfig{PasswordHashed: mustHashPassword(t, "differentpassword")}
				return nil
			},
			wantMatch: true,
		},
		{
			name: "GetConfig returns error — propagated",
			getConfigFunc: func(_ context.Context, _ ...gnmiext.DataElement) error {
				return getConfigErr
			},
			wantErr: true,
		},
		{
			name: "stored hash is invalid — decode error propagated",
			getConfigFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
				elements[0].(*User).Config = &UserConfig{PasswordHashed: "notahash"}
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newProviderWithClient(&gnmiext.ClientMock{
				GetConfigFunc: tt.getConfigFunc,
			})

			got, err := p.hashPassword(t.Context(), username, password)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantHash != "" && got != tt.wantHash {
				t.Errorf("hashPassword() = %q, want %q", got, tt.wantHash)
			}
			if tt.wantMatch {
				d, err := shacrypt.Decode(got)
				if err != nil {
					t.Fatalf("shacrypt.Decode(%q): %v", got, err)
				}
				if !d.Match(password) {
					t.Errorf("hashPassword() produced hash that does not match password")
				}
			}
		})
	}
}
