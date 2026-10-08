// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package connpool provides a protocol-agnostic connection pool that caches
// provider connections across reconciliation cycles.
package connpool

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"sync"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// PooledProvider is an optional interface that providers can implement to
// participate in connection pooling. Providers that do not implement this
// interface will Connect/Disconnect on every reconciliation as before.
type PooledProvider interface {
	// SaveState returns the provider's connection state for caching and an
	// [io.Closer] for the underlying transport. The pool calls Close on
	// eviction or shutdown.
	SaveState() (state any, transport io.Closer)
	// RestoreState restores a previously saved connection state into the
	// provider. It returns false if the state is invalid or stale, in which
	// case the pool will perform a fresh Connect.
	RestoreState(state any) bool
}

// Provider mirrors [provider.Provider] to avoid an import cycle.
type Provider interface {
	Connect(context.Context, *deviceutil.Connection) error
	Disconnect(context.Context, *deviceutil.Connection) error
}

type entry struct {
	state       any
	transport   io.Closer
	fingerprint string
}

// Pool caches provider connection state keyed by device address.
type Pool struct {
	mu      sync.Mutex
	entries map[string]*entry // keyed by address
	devices map[string]string // device name → address
}

// New creates a new connection pool.
func New() *Pool {
	return &Pool{
		entries: make(map[string]*entry),
		devices: make(map[string]string),
	}
}

// Connect establishes a provider connection, returning a cached connection
// when the device's connection parameters have not changed. Providers that
// implement [PooledProvider] participate in caching; all others fall through
// to a direct Connect call every time.
func (p *Pool) Connect(ctx context.Context, prov Provider, conn *deviceutil.Connection) error {
	pooled, ok := prov.(PooledProvider)
	if !ok {
		return prov.Connect(ctx, conn)
	}
	fp := fingerprint(conn)
	p.mu.Lock()
	e, exists := p.entries[conn.Address]
	if exists && e.fingerprint == fp {
		state := e.state
		p.mu.Unlock()
		if pooled.RestoreState(state) {
			return nil
		}
		// State was stale — evict and fall through to reconnect.
		p.Evict(conn.Address)
	} else {
		if exists {
			// Fingerprint changed — close the old connection.
			e.transport.Close()
			delete(p.entries, conn.Address)
		}
		p.mu.Unlock()
	}
	if err := prov.Connect(ctx, conn); err != nil {
		return err
	}
	state, transport := pooled.SaveState()
	p.mu.Lock()
	p.entries[conn.Address] = &entry{
		state:       state,
		transport:   transport,
		fingerprint: fp,
	}
	p.devices[conn.DeviceName] = conn.Address
	p.mu.Unlock()
	return nil
}

// Disconnect is a no-op for providers that implement [PooledProvider] because
// the pool owns the connection lifecycle. Non-pooled providers fall through to
// a direct Disconnect call.
func (p *Pool) Disconnect(ctx context.Context, prov Provider, conn *deviceutil.Connection) error {
	if _, ok := prov.(PooledProvider); ok {
		return nil
	}
	return prov.Disconnect(ctx, conn)
}

// Evict closes and removes the cached connection for the given address.
func (p *Pool) Evict(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evictLocked(address)
}

// EvictByDevice closes and removes the cached connection for the given device name.
func (p *Pool) EvictByDevice(deviceName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if addr, ok := p.devices[deviceName]; ok {
		p.evictLocked(addr)
	}
}

func (p *Pool) evictLocked(address string) {
	e, ok := p.entries[address]
	if !ok {
		return
	}
	e.transport.Close()
	delete(p.entries, address)
	for name, addr := range p.devices {
		if addr == address {
			delete(p.devices, name)
			break
		}
	}
}

// Close closes all cached connections. It is safe to call multiple times.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for addr, e := range p.entries {
		e.transport.Close()
		delete(p.entries, addr)
	}
	clear(p.devices)
	return nil
}

// Start implements [manager.Runnable] so the pool can be added to the
// controller-runtime manager for graceful shutdown.
func (p *Pool) Start(ctx context.Context) error {
	<-ctx.Done()
	return p.Close()
}

func fingerprint(conn *deviceutil.Connection) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%s", conn.Address, conn.Username, conn.Password)
	if conn.TLS != nil {
		for _, cert := range conn.TLS.Certificates {
			for _, raw := range cert.Certificate {
				h.Write(raw)
			}
		}
	}
	return string(h.Sum(nil))
}
