// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package provider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var ErrUnimplemented = errors.New("provider method not implemented")

// Provider is the common interface for creation/deletion of the objects over different drivers.
type Provider interface {
	// CreateDevice call is responsible for Device creation on the provider.
	CreateDevice(context.Context, *v1alpha1.Device, *v1alpha1.ProviderConfig) error
	// DeleteDevice call is responsible for Device deletion on the provider.
	DeleteDevice(context.Context, *v1alpha1.Device) error
	// CreateInterface call is responsible for Interface creation on the provider.
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

// HasProviderConfig checks if the given object has a provider configuration annotation that matches the provided ProviderConfig.
func HasProviderConfig(ctx context.Context, prov *v1alpha1.ProviderConfig, obj metav1.Object) bool {
	name, ok := obj.GetAnnotations()[v1alpha1.ProviderConfigAnnotationName]
	return ok && name != "" && name == prov.Name && prov.GetNamespace() == obj.GetNamespace()
}

// GetProviderConfigFromMetadata retrieves the provider configuration from the metadata annotations of the given object.
func GetProviderConfigFromMetadata(ctx context.Context, r client.Reader, obj metav1.Object) (*v1alpha1.ProviderConfig, error) {
	name, ok := obj.GetAnnotations()[v1alpha1.ProviderConfigAnnotationName]
	if !ok || name == "" {
		return nil, nil
	}
	return GetProviderConfigByName(ctx, r, obj.GetNamespace(), name)
}

// GetProviderConfigByName finds and returns a [v1alpha1.Provider] object using the specified selector.
func GetProviderConfigByName(ctx context.Context, r client.Reader, namespace, name string) (*v1alpha1.ProviderConfig, error) {
	obj := new(v1alpha1.ProviderConfig)
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s", v1alpha1.GroupVersion.WithKind("Provider").String(), name)
	}
	return obj, nil
}
