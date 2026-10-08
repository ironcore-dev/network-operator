// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package connpool

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

type mockTransport struct {
	closed atomic.Bool
}

func (m *mockTransport) Close() error {
	m.closed.Store(true)
	return nil
}

type mockState struct {
	transport *mockTransport
}

type mockPooledProvider struct {
	connectCalls int
	state        *mockState
	connectErr   error
	restoreOK    bool
}

func (m *mockPooledProvider) Connect(_ context.Context, _ *deviceutil.Connection) error {
	m.connectCalls++
	if m.connectErr != nil {
		return m.connectErr
	}
	m.state = &mockState{transport: &mockTransport{}}
	return nil
}

func (m *mockPooledProvider) Disconnect(_ context.Context, _ *deviceutil.Connection) error {
	return nil
}

func (m *mockPooledProvider) SaveState() (any, io.Closer) {
	return m.state, m.state.transport
}

func (m *mockPooledProvider) RestoreState(state any) bool {
	if !m.restoreOK {
		return false
	}
	m.state = state.(*mockState)
	return true
}

type mockPlainProvider struct {
	connectCalls    int
	disconnectCalls int
}

func (m *mockPlainProvider) Connect(_ context.Context, _ *deviceutil.Connection) error {
	m.connectCalls++
	return nil
}

func (m *mockPlainProvider) Disconnect(_ context.Context, _ *deviceutil.Connection) error {
	m.disconnectCalls++
	return nil
}

func conn(address string) *deviceutil.Connection {
	return &deviceutil.Connection{
		DeviceName: "dev1",
		Address:    address,
		Username:   "admin",
		Password:   "secret",
	}
}

func TestPool_CacheHit(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	if prov.connectCalls != 1 {
		t.Fatalf("expected 1 connect call, got %d", prov.connectCalls)
	}
	// Second connect with same fingerprint should restore from cache.
	prov2 := &mockPooledProvider{restoreOK: true}
	if err := pool.Connect(t.Context(), prov2, c); err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if prov2.connectCalls != 0 {
		t.Fatalf("expected 0 connect calls on cache hit, got %d", prov2.connectCalls)
	}
}

func TestPool_FingerprintChange(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	oldTransport := prov.state.transport
	// Change password → different fingerprint.
	c2 := conn("10.0.0.1:57400")
	c2.Password = "new-secret"
	prov2 := &mockPooledProvider{restoreOK: true}
	if err := pool.Connect(t.Context(), prov2, c2); err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if prov2.connectCalls != 1 {
		t.Fatalf("expected fresh connect on fingerprint change, got %d", prov2.connectCalls)
	}
	if !oldTransport.closed.Load() {
		t.Fatal("expected old transport to be closed on fingerprint change")
	}
}

func TestPool_Evict(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("connect: %v", err)
	}
	transport := prov.state.transport
	pool.Evict(c.Address)
	if !transport.closed.Load() {
		t.Fatal("expected transport to be closed after eviction")
	}
	// Next connect should be fresh.
	prov2 := &mockPooledProvider{restoreOK: true}
	if err := pool.Connect(t.Context(), prov2, c); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if prov2.connectCalls != 1 {
		t.Fatalf("expected fresh connect after eviction, got %d", prov2.connectCalls)
	}
}

func TestPool_Close(t *testing.T) {
	pool := New()
	prov := &mockPooledProvider{restoreOK: true}
	c1 := conn("10.0.0.1:57400")
	c2 := conn("10.0.0.2:57400")
	if err := pool.Connect(t.Context(), prov, c1); err != nil {
		t.Fatalf("connect c1: %v", err)
	}
	t1 := prov.state.transport
	prov2 := &mockPooledProvider{restoreOK: true}
	if err := pool.Connect(t.Context(), prov2, c2); err != nil {
		t.Fatalf("connect c2: %v", err)
	}
	t2 := prov2.state.transport
	pool.Close()
	if !t1.closed.Load() || !t2.closed.Load() {
		t.Fatal("expected all transports closed after pool.Close")
	}
}

func TestPool_NonPooledProvider(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPlainProvider{}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if prov.connectCalls != 1 {
		t.Fatalf("expected 1 connect, got %d", prov.connectCalls)
	}
	// Second call should also connect (no caching for plain providers).
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if prov.connectCalls != 2 {
		t.Fatalf("expected 2 connect calls for non-pooled provider, got %d", prov.connectCalls)
	}
	// Disconnect should pass through.
	if err := pool.Disconnect(t.Context(), prov, c); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if prov.disconnectCalls != 1 {
		t.Fatalf("expected 1 disconnect call, got %d", prov.disconnectCalls)
	}
}

func TestPool_DisconnectNoOpForPooled(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("connect: %v", err)
	}
	transport := prov.state.transport
	if err := pool.Disconnect(t.Context(), prov, c); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if transport.closed.Load() {
		t.Fatal("pooled disconnect should not close transport")
	}
}

func TestPool_ConnectError(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{connectErr: errors.New("dial failed")}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err == nil {
		t.Fatal("expected error")
	}
	// Should not have cached anything.
	pool.mu.Lock()
	n := len(pool.entries)
	pool.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 entries after failed connect, got %d", n)
	}
}

func TestPool_StaleRestoreReconnects(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("connect: %v", err)
	}
	oldTransport := prov.state.transport
	// Second provider refuses RestoreState.
	prov2 := &mockPooledProvider{restoreOK: false}
	if err := pool.Connect(t.Context(), prov2, c); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if prov2.connectCalls != 1 {
		t.Fatalf("expected fresh connect when restore fails, got %d", prov2.connectCalls)
	}
	if !oldTransport.closed.Load() {
		t.Fatal("expected old transport closed when restore fails")
	}
}

func TestPool_EvictNonExistent(t *testing.T) {
	pool := New()
	defer pool.Close()
	// Should not panic.
	pool.Evict("nonexistent:57400")
	pool.EvictByDevice("nonexistent")
}

func TestPool_EvictByDevice(t *testing.T) {
	pool := New()
	defer pool.Close()
	prov := &mockPooledProvider{restoreOK: true}
	c := conn("10.0.0.1:57400")
	if err := pool.Connect(t.Context(), prov, c); err != nil {
		t.Fatalf("connect: %v", err)
	}
	transport := prov.state.transport
	pool.EvictByDevice(c.DeviceName)
	if !transport.closed.Load() {
		t.Fatal("expected transport closed after EvictByDevice")
	}
	// Next connect should be fresh.
	prov2 := &mockPooledProvider{restoreOK: true}
	if err := pool.Connect(t.Context(), prov2, c); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if prov2.connectCalls != 1 {
		t.Fatalf("expected fresh connect after EvictByDevice, got %d", prov2.connectCalls)
	}
}
