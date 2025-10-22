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
}

type DeviceProvider interface {
	Provider

	// ListPorts retrieves the list of available ports on the device.
	// This can be used to validate port references in other resources.
	ListPorts(context.Context) ([]DevicePort, error)
	// GetDeviceInfo retrieves basic information about the device,
	// such as manufacturer, model, serial number, and firmware version.
	GetDeviceInfo(context.Context) (*DeviceInfo, error)
}

type DevicePort struct {
	// ID is the unique identifier of the port on the device.
	ID string
	// Type is the type of the port, e.g. "10g".
	Type string
	// SupportedSpeedsGbps is the list of supported speeds for the port in Gbps.
	SupportedSpeedsGbps []int32
	// Trasceiver is the type of transceiver present on the port, e.g. "SFP" or "QSFP", if any.
	Transceiver string
}

type DeviceInfo struct {
	// Manufacturer is the manufacturer of the device, e.g. "Cisco".
	Manufacturer string
	// Model is the model of the device, e.g. "N9K-C9332D-GX2B".
	Model string
	// SerialNumber is the serial number of the device.
	SerialNumber string
	// FirmwareVersion is the firmware version running on the device, e.g. "10.4(3)".
	FirmwareVersion string
}

// InterfaceProvider is the interface for the realization of the Interface objects over different providers.
type InterfaceProvider interface {
	Provider

	// EnsureInterface call is responsible for Interface realization on the provider.
	EnsureInterface(context.Context, *InterfaceRequest) (Result, error)
	// DeleteInterface call is responsible for Interface deletion on the provider.
	DeleteInterface(context.Context, *InterfaceRequest) error
	// GetInterfaceStatus call is responsible for retrieving the current status of the Interface from the provider.
	GetInterfaceStatus(context.Context, *InterfaceRequest) (InterfaceStatus, error)
}

type InterfaceRequest struct {
	Interface      *v1alpha1.Interface
	ProviderConfig *ProviderConfig
}

type InterfaceStatus struct {
	// OperStatus indicates whether the interface is operationally up (true) or down (false).
	OperStatus bool
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

// ManagementAccessProvider is the interface for the realization of the ManagementAccess objects over different providers.
type ManagementAccessProvider interface {
	Provider

	// EnsureManagementAccess call is responsible for ManagementAccess realization on the provider.
	EnsureManagementAccess(context.Context, *EnsureManagementAccessRequest) (Result, error)
	// DeleteManagementAccess call is responsible for ManagementAccess deletion on the provider.
	DeleteManagementAccess(context.Context) error
}

type EnsureManagementAccessRequest struct {
	ManagementAccess *v1alpha1.ManagementAccess
	ProviderConfig   *ProviderConfig
}

// ISISProvider is the interface for the realization of the ISIS objects over different providers.
type ISISProvider interface {
	Provider

	// EnsureISIS call is responsible for ISIS realization on the provider.
	EnsureISIS(context.Context, *EnsureISISRequest) (Result, error)
	// DeleteISIS call is responsible for ISIS deletion on the provider.
	DeleteISIS(context.Context, *DeleteISISRequest) error
}

type EnsureISISRequest struct {
	ISIS           *v1alpha1.ISIS
	Interfaces     []ISISInterface
	ProviderConfig *ProviderConfig
}

type ISISInterface struct {
	Interface *v1alpha1.Interface
	BFD       bool
}

type DeleteISISRequest struct {
	ISIS           *v1alpha1.ISIS
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
