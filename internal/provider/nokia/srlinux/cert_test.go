// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

// newTestCert generates a self-signed ECDSA certificate and returns
// its DER bytes and the private key.
func newTestCert(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return der, key
}

// recordingClient is a gnmiext.Client stub that records Update and Delete calls.
type recordingClient struct {
	gnmiext.Client
	updates int
	deletes int
}

func (c *recordingClient) Update(_ context.Context, _ ...gnmiext.DataElement) error {
	c.updates++
	return nil
}

func (c *recordingClient) Delete(_ context.Context, _ ...gnmiext.DataElement) error {
	c.deletes++
	return nil
}

func TestEnsureCertificate(t *testing.T) {
	der1, key1 := newTestCert(t)
	der2, _ := newTestCert(t)

	tests := []struct {
		name       string
		getConfig  func(context.Context, ...gnmiext.DataElement) error
		wantUpdate bool
	}{
		{
			name: "profile absent — writes certificate",
			getConfig: func(_ context.Context, _ ...gnmiext.DataElement) error {
				return gnmiext.ErrNil
			},
			wantUpdate: true,
		},
		{
			name: "profile present with matching certificate — no write",
			getConfig: func(_ context.Context, elems ...gnmiext.DataElement) error {
				elems[0].(*TLSProfile).Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der1}))
				// key comes back AES-encrypted on real device; simulate that
				elems[0].(*TLSProfile).Key = "$aes1$fake-encrypted-key"
				return nil
			},
			wantUpdate: false,
		},
		{
			name: "profile present with different certificate — writes certificate",
			getConfig: func(_ context.Context, elems ...gnmiext.DataElement) error {
				elems[0].(*TLSProfile).Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der2}))
				elems[0].(*TLSProfile).Key = "$aes1$fake-encrypted-key"
				return nil
			},
			wantUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &recordingClient{}
			rc.Client = &stubClient{getConfigFn: tt.getConfig}
			p := &Provider{client: rc}

			req := &provider.EnsureCertificateRequest{
				ID: "test-profile",
				Certificate: &tls.Certificate{
					Certificate: [][]byte{der1},
					PrivateKey:  key1,
				},
			}

			if err := p.EnsureCertificate(t.Context(), req); err != nil {
				t.Fatalf("EnsureCertificate: %v", err)
			}

			if got := rc.updates > 0; got != tt.wantUpdate {
				t.Errorf("Update called = %v, want %v", got, tt.wantUpdate)
			}
		})
	}
}

func TestEnsureCertificateIdempotent(t *testing.T) {
	der, key := newTestCert(t)

	callCount := 0
	rc := &recordingClient{}
	rc.Client = &stubClient{
		getConfigFn: func(_ context.Context, elems ...gnmiext.DataElement) error {
			if callCount == 0 {
				return gnmiext.ErrNil
			}
			elems[0].(*TLSProfile).Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
			elems[0].(*TLSProfile).Key = "$aes1$fake-encrypted-key"
			return nil
		},
	}
	p := &Provider{client: rc}

	req := &provider.EnsureCertificateRequest{
		ID: "idempotent-profile",
		Certificate: &tls.Certificate{
			Certificate: [][]byte{der},
			PrivateKey:  key,
		},
	}

	for range 2 {
		if err := p.EnsureCertificate(t.Context(), req); err != nil {
			t.Fatalf("EnsureCertificate call %d: %v", callCount+1, err)
		}
		callCount++
	}

	if rc.updates != 1 {
		t.Errorf("Update called %d times, want 1", rc.updates)
	}
}

func TestDeleteCertificate(t *testing.T) {
	rc := &recordingClient{Client: &stubClient{}}
	p := &Provider{client: rc}

	if err := p.DeleteCertificate(t.Context(), &provider.DeleteCertificateRequest{ID: "profile"}); err != nil {
		t.Fatalf("DeleteCertificate: %v", err)
	}
	if rc.deletes != 1 {
		t.Errorf("Delete called %d times, want 1", rc.deletes)
	}
}

func TestEnsureCertificateChain(t *testing.T) {
	leaf, key := newTestCert(t)
	ca1, _ := newTestCert(t)
	ca2, _ := newTestCert(t)

	tests := []struct {
		name       string
		installed  [][]byte
		wantUpdate bool
	}{
		{
			name:       "leaf and CA both match — no write",
			installed:  [][]byte{leaf, ca1},
			wantUpdate: false,
		},
		{
			name:       "leaf matches but CA differs — writes certificate",
			installed:  [][]byte{leaf, ca2},
			wantUpdate: true,
		},
		{
			name:       "leaf matches but CA missing — writes certificate",
			installed:  [][]byte{leaf},
			wantUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &recordingClient{}
			rc.Client = &stubClient{getConfigFn: func(_ context.Context, elems ...gnmiext.DataElement) error {
				elems[0].(*TLSProfile).Certificate = string(certChainPEM(tt.installed))
				elems[0].(*TLSProfile).Key = "$aes1$fake-encrypted-key"
				return nil
			}}
			p := &Provider{client: rc}

			req := &provider.EnsureCertificateRequest{
				ID: "chain-profile",
				Certificate: &tls.Certificate{
					Certificate: [][]byte{leaf, ca1},
					PrivateKey:  key,
				},
			}

			if err := p.EnsureCertificate(t.Context(), req); err != nil {
				t.Fatalf("EnsureCertificate: %v", err)
			}

			if got := rc.updates > 0; got != tt.wantUpdate {
				t.Errorf("Update called = %v, want %v", got, tt.wantUpdate)
			}
		})
	}
}

// stubClient embeds gnmiext.Client and overrides GetConfig.
type stubClient struct {
	gnmiext.Client
	getConfigFn func(context.Context, ...gnmiext.DataElement) error
}

func (s *stubClient) GetConfig(ctx context.Context, elems ...gnmiext.DataElement) error {
	if s.getConfigFn != nil {
		return s.getConfigFn(ctx, elems...)
	}
	return gnmiext.ErrNil
}

func (s *stubClient) Update(_ context.Context, _ ...gnmiext.DataElement) error { return nil }
func (s *stubClient) Delete(_ context.Context, _ ...gnmiext.DataElement) error { return nil }
