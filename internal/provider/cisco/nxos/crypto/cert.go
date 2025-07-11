// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/openconfig/gnoi/cert"
	"google.golang.org/grpc"
)

type Certificate struct {
	Key  *rsa.PrivateKey
	Cert *x509.Certificate
}

// Load loads the certificate into the specified trustpoint via the gNOI [cert service].
//
// [cert service]: https://github.com/openconfig/gnoi/blob/main/cert/cert.proto
func (c *Certificate) Load(ctx context.Context, conn *grpc.ClientConn, trustpoint string) error {
	b, err := c.Encode()
	if err != nil {
		return err
	}

	priv, pub, err := c.EncodeKeyPair()
	if err != nil {
		return err
	}

	// Only the `LoadCertificate` method is currently supported on the Nexus 9000 series. Even though it's already deprecated...
	// See: https://www.cisco.com/c/en/us/td/docs/dcn/nx-os/nexus9000/104x/programmability/cisco-nexus-9000-series-nx-os-programmability-guide-104x/gnoi---operation-interface.html
	_, err = cert.NewCertificateManagementClient(conn).LoadCertificate(ctx, &cert.LoadCertificateRequest{ //nolint:staticcheck
		Certificate:   &cert.Certificate{Type: cert.CertificateType_CT_X509, Certificate: b},
		KeyPair:       &cert.KeyPair{PrivateKey: priv, PublicKey: pub},
		CertificateId: trustpoint,
	}, grpc.WaitForReady(true))
	return err
}

func (c *Certificate) Encode() ([]byte, error) {
	der, err := x509.CreateCertificate(rand.Reader, c.Cert, c.Cert, &c.Key.PublicKey, c.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *Certificate) EncodeKeyPair() (private, public []byte, err error) {
	var priv bytes.Buffer
	err = pem.Encode(&priv, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(c.Key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %w", err)
	}
	b, err := x509.MarshalPKIXPublicKey(&c.Key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	var pub bytes.Buffer
	err = pem.Encode(&pub, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: b,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	return priv.Bytes(), pub.Bytes(), nil
}
