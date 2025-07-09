// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewClient(ctx context.Context, target, username, password string) (*ygnmi.Client, io.Closer, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		})),
		grpc.WithPerRPCCredentials(&auth{
			username: username,
			password: password,
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create grpc client: %w", err)
	}
	client, err := ygnmi.NewClient(gpb.NewGNMIClient(conn), ygnmi.WithRequestLogLevel(6))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ygnmi client: %w", err)
	}
	return client, conn, nil
}

type auth struct {
	username string
	password string
}

var _ credentials.PerRPCCredentials = (*auth)(nil)

func (a *auth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"username": a.username,
		"password": a.password,
	}, nil
}

func (a *auth) RequireTransportSecurity() bool { return true }
