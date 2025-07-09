// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package provider

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

// Provider is the common interface for creation/deletion of the objects over different drivers.
type Provider interface {
	// CreateInterface call is responsible for Interface creation on the provider
	CreateInterface(context.Context, *v1alpha1.Interface) error
	// DeleteInterface call is responsible for Interface deletion on the provider.
	DeleteInterface(context.Context, *v1alpha1.Interface) error
}

var mu sync.RWMutex

// providers holds all registered providers.
// It should be accessed in a thread-safe manner and kept private to this package.
var providers = make(map[string]Provider)

// Register registers a new provider with the given name.
// If a provider with the same name already exists, it panics.
func Register(name string, provider Provider) {
	mu.Lock()
	defer mu.Unlock()
	if providers == nil {
		panic("Register provider is nil")
	}
	if _, ok := providers[name]; ok {
		panic("Register called twice for provider " + name)
	}
	providers[name] = provider
}

// Get returns the provider with the given name.
// If the provider does not exist, it returns an error.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	provider, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return provider, nil
}

// Providers returns a slice of all registered provider names.
func Providers() []string {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Sorted(maps.Keys(providers))
}
