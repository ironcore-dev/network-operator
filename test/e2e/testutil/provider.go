// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/iosxr"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
)

// E2ETestLabel is the label key applied to resources created by e2e tests for cleanup tracking.
const E2ETestLabel = "networking.metal.ironcore.dev/e2e-test"

// ProviderType represents the network device provider to test against.
type ProviderType string

// ProviderFactory creates a new provider instance.
type ProviderFactory = func() provider.Provider

// Provider names must match the registered provider names in internal/provider/*/provider.go
const (
	ProviderNXOS  ProviderType = "cisco-nxos-gnmi"
	ProviderIOSXR ProviderType = "cisco-iosxr-gnmi"
)

// ProviderConfig holds the configuration for a provider test.
type ProviderConfig struct {
	Name        ProviderType
	NewProvider ProviderFactory
}

// SupportedProviders lists all providers to test.
var SupportedProviders = []ProviderConfig{
	{Name: ProviderNXOS, NewProvider: func() provider.Provider { return nxos.NewProvider() }},
	{Name: ProviderIOSXR, NewProvider: func() provider.Provider { return iosxr.NewProvider() }},
}

// CoreResources are the main API resources with finalizers.
// During cleanup, these are deleted FIRST so their finalizers can
// complete while Device and config resources still exist.
var CoreResources = []schema.GroupVersionKind{
	v1alpha1.GroupVersion.WithKind("Interface"),
	v1alpha1.GroupVersion.WithKind("VLAN"),
	v1alpha1.GroupVersion.WithKind("VRF"),
	v1alpha1.GroupVersion.WithKind("NTP"),
	v1alpha1.GroupVersion.WithKind("DNS"),
	v1alpha1.GroupVersion.WithKind("LLDP"),
	v1alpha1.GroupVersion.WithKind("Banner"),
	v1alpha1.GroupVersion.WithKind("OSPF"),
	v1alpha1.GroupVersion.WithKind("PIM"),
	v1alpha1.GroupVersion.WithKind("NetworkVirtualizationEdge"),
	v1alpha1.GroupVersion.WithKind("EVPNInstance"),
	v1alpha1.GroupVersion.WithKind("RoutingPolicy"),
	v1alpha1.GroupVersion.WithKind("PrefixSet"),
	v1alpha1.GroupVersion.WithKind("BGP"),
	v1alpha1.GroupVersion.WithKind("BGPPeer"),
	v1alpha1.GroupVersion.WithKind("Syslog"),
	v1alpha1.GroupVersion.WithKind("SNMP"),
	v1alpha1.GroupVersion.WithKind("ManagementAccess"),
	v1alpha1.GroupVersion.WithKind("AccessControlList"),
	v1alpha1.GroupVersion.WithKind("DHCPRelay"),
	v1alpha1.GroupVersion.WithKind("ISIS"),
}

// ConfigResources are provider-specific config resources (e.g., NX-OS configs).
// During cleanup, these are deleted AFTER core resources.
var ConfigResources = []schema.GroupVersionKind{
	nxv1alpha1.GroupVersion.WithKind("InterfaceConfig"),
	nxv1alpha1.GroupVersion.WithKind("LLDPConfig"),
	nxv1alpha1.GroupVersion.WithKind("BGPConfig"),
	nxv1alpha1.GroupVersion.WithKind("VPCDomain"),
}

// ResourcePluralName returns the plural resource name for a GVK.
// These must match the CRD spec.names.plural values (from `kubectl api-resources`).
// We can't use meta.UnsafeGuessKindToResource because CRDs define their own plurals
// which don't always follow standard Kubernetes pluralization rules.
func ResourcePluralName(gvk schema.GroupVersionKind) string {
	plurals := map[string]string{
		"Interface":                 "interfaces",
		"VLAN":                      "vlans",
		"VRF":                       "vrfs",
		"NTP":                       "ntp",
		"DNS":                       "dns",
		"LLDP":                      "lldps",
		"Banner":                    "banners",
		"OSPF":                      "ospf",
		"PIM":                       "pim",
		"NetworkVirtualizationEdge": "networkvirtualizationedges",
		"EVPNInstance":              "evpninstances",
		"InterfaceConfig":           "interfaceconfigs",
		"LLDPConfig":                "lldpconfigs",
		"VPCDomain":                 "vpcdomains",
		"BGPConfig":                 "bgpconfigs",
		"RoutingPolicy":             "routingpolicies",
		"PrefixSet":                 "prefixsets",
		"BGP":                       "bgp",
		"BGPPeer":                   "bgppeers",
		"Syslog":                    "syslogs",
		"SNMP":                      "snmp",
		"ManagementAccess":          "managementaccesses",
		"AccessControlList":         "accesscontrollists",
		"DHCPRelay":                 "dhcprelays",
		"ISIS":                      "isis",
		"Device":                    "devices",
	}
	if plural, ok := plurals[gvk.Kind]; ok {
		return plural
	}
	// Fallback to standard pluralization
	plural, _ := meta.UnsafeGuessKindToResource(gvk)
	return plural.Resource
}

// CreateTestDevice creates a Device pointing to the gNMI server with a generated name.
func CreateTestDevice(ctx context.Context, c client.Client, gnmiAddr, namespace string) (*v1alpha1.Device, error) {
	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-device-",
			Namespace:    namespace,
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: v1alpha1.Endpoint{
				Address: gnmiAddr,
			},
		},
	}
	if err := c.Create(ctx, device); err != nil {
		return nil, err
	}

	// Set the device status to Running so that dependent resources can reconcile
	device.Status.Phase = v1alpha1.DevicePhaseRunning
	if err := c.Status().Update(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

// CleanupTimeout is the timeout for cleanup operations.
const CleanupTimeout = 30 * time.Second

// CleanupInterval is the polling interval for cleanup operations.
const CleanupInterval = 100 * time.Millisecond
