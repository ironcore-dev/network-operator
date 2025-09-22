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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	// "github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// Provider is the common interface used to establish and tear down connections to the provider.
type Provider interface {
	Init(context.Context) error
	// disconnect(context.Context, *ProviderConfig) error
}

type Result struct {
	// RequeueAfter if greater than 0, indicates that the caller should retry the request after the specified duration.
	// This is useful for situations where the operation is pending and needs to be retried later.
	RequeueAfter time.Duration
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
type ProviderFunc func(runtimeConfig ProviderInitConfig) Provider

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
func GetProviderByName(name string) (ProviderFunc, error) {
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
func GetProviderConfig(ctx context.Context, r client.Reader, namespace string, ref *v1alpha1.ProviderConfigReference) (*ProviderConfig, error) {
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
	config *unstructured.Unstructured
}

// Into converts the underlying unstructured object into the specified type.
func (p ProviderConfig) GetConfig(v any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(p.config.Object, v)
}

type ProviderInitConfig interface {
	GetProviderType() string
}


func GetInterfaceProvider(ctx context.Context, r client.Reader, iface *v1alpha1.Interface) (InterfaceProvider, error) {
	device, err := deviceutil.GetDeviceFromMetadata(ctx, r, iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get device for interface %s/%s: %w", iface.Namespace, iface.Name, err)
	}

	provider, err := GetProvider(ctx, r, device)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	ifaceProvider, ok := provider.(InterfaceProvider)
	if !ok {
		return nil, fmt.Errorf("provider does not implement provider.InterfaceProvider")
	}
	return ifaceProvider, nil
}

func GetProvider(ctx context.Context,r client.Reader, device *v1alpha1.Device) (Provider, error) {
	providerFunc, err := GetProviderByName(device.Spec.ProviderType)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	switch device.Spec.ProviderType {
	case v1alpha1.OpenconfigProviderType:
		cfg, err := GetProviderConfig(ctx, r, device.Namespace, device.Spec.ProviderConfigRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider config: %w", err)
		}

		openconfigCfg := &v1alpha1.OpenconfigProviderConfig{}
		if err := cfg.GetConfig(openconfigCfg); err != nil {
			return nil, fmt.Errorf("failed to convert provider config: %w", err)
		}


		user, pass, tls, err := deviceutil.GetDeviceSecureConnection(ctx, r, device, openconfigCfg.Spec.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to get device connection: %w", err)
		}
		initCfg := &OpenconfigProviderInitConfig{
			ProviderType: v1alpha1.OpenconfigProviderType,
			Address:      openconfigCfg.Spec.Endpoint.Address,
			Username:     user,
			Password:     pass,
			TLS:          tls,
		}
		prov := providerFunc(initCfg)
		return prov, nil

	case v1alpha1.SonicProviderType:
		cfg, err := GetProviderConfig(ctx, r, device.Namespace, device.Spec.ProviderConfigRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider config: %w", err)
		}

		sonicCfg := &v1alpha1.SonicProviderConfig{}
		if err := cfg.GetConfig(sonicCfg); err != nil {
			return nil, fmt.Errorf("failed to convert provider config: %w", err)
		}

		initCfg := &SonicProviderInitConfig{
			ProviderType: v1alpha1.SonicProviderType,
			Address:      sonicCfg.Spec.Address,
			Port:         sonicCfg.Spec.Port,
		}
		prov := providerFunc(initCfg)
		return prov, nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", device.Spec.ProviderType)
	}
}


func ModifyFinalizerOnProviderConfig(ctx context.Context, c client.Client, device *v1alpha1.Device, finalizer string, operation int) error {
	switch device.Spec.ProviderType {
	case v1alpha1.OpenconfigProviderType:
		cfg, err := GetProviderConfig(ctx, c, device.Namespace, device.Spec.ProviderConfigRef)
		if err != nil {
			return fmt.Errorf("failed to get provider config: %w", err)
		}

		openconfigCfg := &v1alpha1.OpenconfigProviderConfig{}
		if err := cfg.GetConfig(openconfigCfg); err != nil {
			return fmt.Errorf("failed to convert provider config: %w", err)
		}

		if err := clientutil.ModifyFinalizerOnObject(ctx, c, openconfigCfg, finalizer, operation); err != nil {
			return fmt.Errorf("failed to modify finalizer on provider config: %w", err)
		}

		if ref := openconfigCfg.Spec.Endpoint.SecretRef; ref != nil {
			secret := new(corev1.Secret)
			if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, secret); err != nil {
				return fmt.Errorf("failed to get endpoint secret for provider config: %w", err)
			}

			if err := clientutil.ModifyFinalizerOnObject(ctx, c, secret, finalizer, operation); err != nil {
				return fmt.Errorf("failed to modify finalizer on endpoint secret: %w", err)
			}
		}

	case v1alpha1.SonicProviderType:
		cfg, err := GetProviderConfig(ctx, c, device.Namespace, device.Spec.ProviderConfigRef)
		if err != nil {
			return fmt.Errorf("failed to get provider config: %w", err)
		}

		sonicCfg := &v1alpha1.SonicProviderConfig{}
		if err := cfg.GetConfig(sonicCfg); err != nil {
			return fmt.Errorf("failed to convert provider config: %w", err)
		}

		if err := clientutil.ModifyFinalizerOnObject(ctx, c, sonicCfg, finalizer, operation); err != nil {
			return fmt.Errorf("failed to modify finalizer on provider config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported provider type: %s", device.Spec.ProviderType)
	}

	return nil
}
