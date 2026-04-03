// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package grpcext provides convenience functions and types for working with gRPC clients.
package grpcext

import (
	"context"
	"errors"
	"slices"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// NewClient creates a new gRPC client connection to a specified device using the provided [deviceutil.Connection].
// The connection will use TLS if the [deviceutil.Connection.TLS] field is set, otherwise it will use an insecure connection.
// If the [deviceutil.Connection.Username] and [deviceutil.Connection.Password] fields are set, basic authentication in the form of metadata will be used.
func NewClient(ctx context.Context, conn *deviceutil.Connection, o ...Option) (*grpc.ClientConn, error) {
	creds := insecure.NewCredentials()
	if conn.TLS != nil {
		creds = credentials.NewTLS(conn.TLS)
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds), grpc.WithUnaryInterceptor(TerminalErrorInterceptor())}
	if conn.Username != "" && conn.Password != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&auth{
			Username: conn.Username,
			Password: conn.Password,
		}))
	}

	for _, opt := range o {
		dialOpt, err := opt()
		if err != nil {
			return nil, err
		}
		opts = append(opts, dialOpt)
	}

	return grpc.NewClient(conn.Address, opts...)
}

type Option func() (grpc.DialOption, error)

// WithDefaultTimeout returns a gRPC dial option that sets a default timeout for each RPC.
// If a deadline is already present in the context, it will not be modified.
func WithDefaultTimeout(timeout time.Duration) Option {
	return func() (grpc.DialOption, error) {
		if timeout <= 0 {
			return nil, errors.New("timeout must be greater than zero")
		}
		return grpc.WithUnaryInterceptor(UnaryDefaultTimeoutInterceptor(timeout)), nil
	}
}

type auth struct {
	Username string
	Password string `json:"-"`
}

var _ credentials.PerRPCCredentials = (*auth)(nil)

func (a *auth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"username": a.Username,
		"password": a.Password,
	}, nil
}

func (a *auth) RequireTransportSecurity() bool {
	// Only called if the transport credentials are insecure.
	return false
}

// UnaryDefaultTimeoutInterceptor returns a gRPC unary client interceptor that sets a default timeout
// for each RPC. If a deadline is already present, it will not be modified.
func UnaryDefaultTimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); ok {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// TerminalErrorInterceptor returns a gRPC unary client interceptor that wraps errors returned by the gRPC invoker
// as terminal errors if their gRPC status code is in the set of non-retryable codes defined in [terminalCodes].
func TerminalErrorInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return WrapTerminalError(invoker(ctx, method, req, reply, cc, opts...))
	}
}

// WrapTerminalError wraps the given error as a terminal error if its gRPC status error
// with a non-retryable code.
func WrapTerminalError(err error) error {
	if statusErr, ok := status.FromError(err); ok && slices.Contains(terminalCodes, statusErr.Code()) {
		return reconcile.TerminalError(err)
	}
	return err
}

// terminalCodes holds the set of gRPC codes that are considered terminal.
// That is, if an error has one of these codes, retrying the operation
// is not expected to succeed.
// This list is based on the gRPC documentation at https://grpc.io/docs/guides/status-codes.
var terminalCodes = []codes.Code{
	codes.Unknown,
	codes.InvalidArgument,
	codes.NotFound,
	codes.AlreadyExists,
	codes.PermissionDenied,
	codes.FailedPrecondition,
	codes.OutOfRange,
	codes.Unimplemented,
	codes.DataLoss,
	codes.Unauthenticated,
}
