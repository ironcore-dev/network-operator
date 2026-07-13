// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"crypto/tls"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewGNMIClient creates a gNMI client connected to the given address.
// It uses TLS with certificate verification disabled for testing.
func NewGNMIClient(ctx context.Context, addr string) (gpb.GNMIClient, *grpc.ClientConn, error) {
	creds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // Test client connecting to test server
	})

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, nil, err
	}

	return gpb.NewGNMIClient(conn), conn, nil
}

// MustNewGNMIClient creates a gNMI client and panics on error.
// Useful in BeforeSuite where Expect is not available.
func MustNewGNMIClient(ctx context.Context, addr string) (gpb.GNMIClient, *grpc.ClientConn) {
	client, conn, err := NewGNMIClient(ctx, addr)
	if err != nil {
		panic(err)
	}
	return client, conn
}
