// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxapi

import (
	"context"
	"encoding/json"
)

// NewClientMock returns a [nxapi.ClientMock] whose Do method is implemented by the given function.
func NewClientMock(doFunc func(context.Context, Request) ([]json.RawMessage, error)) *ClientMock {
	client := &ClientMock{
		DoFunc: doFunc,
	}
	client.CloneFunc = func(...Option) (Client, error) { return client, nil }
	return client
}

// MockErrorClient returns an [nxapi.ClientMock] whose Do always fails with a single
// [nxapi.RPCError] carrying the given code and message, so tests can exercise
// how helpers react to device-side command failures.
func MockErrorClient(code int, message string) *ClientMock {
	return NewClientMock(func(_ context.Context, _ Request) ([]json.RawMessage, error) {
		return nil, RPCErrors{{Code: code, Message: message}}
	})
}
