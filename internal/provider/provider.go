// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package provider

import (
	"context"
	"crypto/tls"
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

// BannerProvider is the interface for the realization of the Banner objects over different providers.
type BannerProvider interface {
	Provider

	// EnsureBanner call is responsible for Banner realization on the provider.
	EnsureBanner(context.Context, *BannerRequest) (Result, error)
	// DeleteBanner call is responsible for Banner deletion on the provider.
	DeleteBanner(context.Context) error
}

type BannerRequest struct {
	Message        string
	ProviderConfig *ProviderConfig
}

// UserProvider is the interface for the realization of the User objects over different providers.
type UserProvider interface {
	Provider

	// EnsureUser call is responsible for User realization on the provider.
	EnsureUser(context.Context, *EnsureUserRequest) (Result, error)
	// DeleteUser call is responsible for User deletion on the provider.
	DeleteUser(context.Context, *DeleteUserRequest) error
}

type EnsureUserRequest struct {
	Username       string
	Password       string
	SSHKey         string
	Roles          []string
	ProviderConfig *ProviderConfig
}

type DeleteUserRequest struct {
	Username       string
	ProviderConfig *ProviderConfig
}

// DNSProvider is the interface for the realization of the DNS objects over different providers.
type DNSProvider interface {
	Provider

	// EnsureDNS call is responsible for DNS realization on the provider.
	EnsureDNS(context.Context, *EnsureDNSRequest) (Result, error)
	// DeleteDNS call is responsible for DNS deletion on the provider.
	DeleteDNS(context.Context) error
}

type EnsureDNSRequest struct {
	DNS            *v1alpha1.DNS
	ProviderConfig *ProviderConfig
}

// NTPProvider is the interface for the realization of the NTP objects over different providers.
type NTPProvider interface {
	Provider

	// EnsureNTP call is responsible for NTP realization on the provider.
	EnsureNTP(context.Context, *EnsureNTPRequest) (Result, error)
	// DeleteNTP call is responsible for NTP deletion on the provider.
	DeleteNTP(context.Context) error
}

type EnsureNTPRequest struct {
	NTP            *v1alpha1.NTP
	ProviderConfig *ProviderConfig
}

// ACLProvider is the interface for the realization of the AccessControlList objects over different providers.
type ACLProvider interface {
	Provider

	// EnsureACL call is responsible for AccessControlList realization on the provider.
	EnsureACL(context.Context, *EnsureACLRequest) (Result, error)
	// DeleteACL call is responsible for AccessControlList deletion on the provider.
	DeleteACL(context.Context, *DeleteACLRequest) error
}

type EnsureACLRequest struct {
	ACL            *v1alpha1.AccessControlList
	ProviderConfig *ProviderConfig
}

type DeleteACLRequest struct {
	Name           string
	ProviderConfig *ProviderConfig
}

// CertificateProvider is the interface for the realization of the Certificate objects over different providers.
type CertificateProvider interface {
	Provider

	// EnsureCertificate call is responsible for Certificate realization on the provider.
	EnsureCertificate(context.Context, *EnsureCertificateRequest) (Result, error)
	// DeleteCertificate call is responsible for Certificate deletion on the provider.
	DeleteCertificate(context.Context, *DeleteCertificateRequest) error
}

type EnsureCertificateRequest struct {
	ID             string
	Certificate    *tls.Certificate
	ProviderConfig *ProviderConfig
}

type DeleteCertificateRequest struct {
	ID             string
	ProviderConfig *ProviderConfig
}

// SNMPProvider is the interface for the realization of the SNMP objects over different providers.
type SNMPProvider interface {
	Provider

	// EnsureSNMP call is responsible for SNMP realization on the provider.
	EnsureSNMP(context.Context, *EnsureSNMPRequest) (Result, error)
	// DeleteSNMP call is responsible for SNMP deletion on the provider.
	DeleteSNMP(context.Context, *DeleteSNMPRequest) error
}

type EnsureSNMPRequest struct {
	SNMP           *v1alpha1.SNMP
	ProviderConfig *ProviderConfig
}

type DeleteSNMPRequest struct {
	ProviderConfig *ProviderConfig
}

// SyslogProvider is the interface for the realization of the Syslog objects over different providers.
type SyslogProvider interface {
	Provider

	// EnsureSyslog call is responsible for Syslog realization on the provider.
	EnsureSyslog(context.Context, *EnsureSyslogRequest) (Result, error)
	// DeleteSyslog call is responsible for Syslog deletion on the provider.
	DeleteSyslog(context.Context) error
}

type EnsureSyslogRequest struct {
	Syslog         *v1alpha1.Syslog
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
