// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ provider.CertificateProvider = (*Provider)(nil)
	_ gnmiext.DataElement          = (*TLSProfile)(nil)
)

func (p *Provider) EnsureCertificate(ctx context.Context, req *provider.EnsureCertificateRequest) error {
	current := &TLSProfile{Name: req.ID}
	err := p.client.GetConfig(ctx, current)
	if err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}
	if err == nil && chainMatches(current.Certificate, req.Certificate.Certificate) {
		logr.FromContextOrDiscard(ctx).V(2).Info("Certificate already installed", "id", req.ID)
		return nil
	}
	keyPEM, err := pemEncodeKey(req.Certificate.PrivateKey)
	if err != nil {
		return fmt.Errorf("encoding private key: %w", err)
	}
	return p.client.Update(ctx, &TLSProfile{
		Name:        req.ID,
		Key:         string(keyPEM),
		Certificate: string(certChainPEM(req.Certificate.Certificate)),
	})
}

func (p *Provider) DeleteCertificate(ctx context.Context, req *provider.DeleteCertificateRequest) error {
	return p.client.Delete(ctx, &TLSProfile{Name: req.ID})
}

// TLSProfile maps the SR Linux TLS profile container.
type TLSProfile struct {
	Name               string `json:"name"`
	Key                string `json:"key"`
	Certificate        string `json:"certificate"`
	AuthenticateClient bool   `json:"authenticate-client"`
}

func (t *TLSProfile) XPath() string {
	return fmt.Sprintf("srl_nokia-system:system/srl_nokia-tls:tls/profile[name=%s]", t.Name)
}

// chainMatches reports whether the installed PEM certificate chain matches the
// desired DER chain (leaf followed by any intermediates/CAs). The key field is
// AES-encrypted on read and cannot be used for comparison, so only the
// certificate chain is checked.
func chainMatches(installedPEM string, wantDERs [][]byte) bool {
	rest := []byte(installedPEM)
	for _, want := range wantDERs {
		block, remaining := pem.Decode(rest)
		if block == nil || !bytes.Equal(block.Bytes, want) {
			return false
		}
		rest = remaining
	}
	extra, _ := pem.Decode(rest)
	return extra == nil
}

func pemEncodeKey(key crypto.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func certChainPEM(ders [][]byte) []byte {
	var buf bytes.Buffer
	for _, der := range ders {
		buf.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	return buf.Bytes()
}
