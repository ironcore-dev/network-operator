// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/transport/grpcext"
)

var (
	_ provider.Provider       = (*Provider)(nil)
	_ provider.DeviceProvider = (*Provider)(nil)
)

// timeout is the default timeout for all gRPC requests made by the provider.
const timeout = 30 * time.Second

type Provider struct {
	conn   *grpc.ClientConn
	client gnmiext.Client
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func (p *Provider) Connect(ctx context.Context, conn *deviceutil.Connection) (err error) {
	p.conn, err = grpcext.NewClient(conn, grpcext.WithDefaultTimeout(timeout))
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}

	var opts []gnmiext.Option
	if logger, err := logr.FromContext(ctx); err == nil && !logger.IsZero() {
		opts = append(opts, gnmiext.WithLogger(logger))
	}
	p.client, err = gnmiext.New(ctx, p.conn, opts...)
	if err != nil {
		return fmt.Errorf("failed to create gnmi client: %w", err)
	}
	return nil
}

func (p *Provider) Disconnect(_ context.Context, _ *deviceutil.Connection) error {
	return p.conn.Close()
}

func init() {
	provider.Register("srlinux.nokia.networking.metal.ironcore.dev", NewProvider)
}
