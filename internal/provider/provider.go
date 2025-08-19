// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package provider

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// Provider is the common interface used to establish and tear down connections to the provider.
type Provider interface {
	Connect(context.Context, *deviceutil.Connection) error
	Disconnect(context.Context, *deviceutil.Connection) error
}

type Result struct {
	// RequeueAfter if greater than 0, indicates that the caller should retry the request after the specified duration.
	// This is useful for situations where the operation is pending and needs to be retried later.
	RequeueAfter time.Duration

	// Conditions contains the conditions that should be applied to the resource's status.
	// The caller is responsible for merging these conditions with any existing conditions on the resource.
	Conditions []metav1.Condition
}

// InterfaceProvider is the interface for the realization of the Interface objects over different providers.
type InterfaceProvider interface {
	Provider

	// EnsureInterface call is responsible for Interface realization on the provider.
	EnsureInterface(context.Context, *InterfaceRequest) (Result, error)
	// DeleteInterface call is responsible for Interface deletion on the provider.
	DeleteInterface(context.Context, *InterfaceRequest) error
}

type InterfaceRequest struct {
	Interface      *v1alpha1.Interface
	ProviderConfig *ProviderConfig
}

var mu sync.RWMutex

// ProviderFunc returns a new [Provider] instance.
type ProviderFunc func() Provider

// providers holds all registered providers.
// It should be accessed in a thread-safe manner and kept private to this package.
var providers = make(map[string]ProviderFunc)

// Register registers a new provider with the given name.
// If a provider with the same name already exists, it panics.
func Register(name string, provider ProviderFunc) {
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
func Get(name string) (ProviderFunc, error) {
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

// GetProviderConfig retrieves the provider-specific configuration resource for a given reference.
func GetProviderConfig(ctx context.Context, r client.Reader, namespace string, ref *v1alpha1.TypedLocalObjectReference) (*ProviderConfig, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	obj.SetNamespace(namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, fmt.Errorf("failed to get provider config %s/%s (%s): %w", namespace, ref.Name, obj.GetObjectKind().GroupVersionKind().String(), err)
	}
	return &ProviderConfig{obj}, nil
}

// ProviderConfig is a wrapper around an [unstructured.Unstructured] object that represents a provider-specific configuration.
type ProviderConfig struct {
	obj *unstructured.Unstructured
}

// Into converts the underlying unstructured object into the specified type.
func (p ProviderConfig) Into(v any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(p.obj.Object, v)
}
