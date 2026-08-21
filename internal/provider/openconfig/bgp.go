// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.BGPProvider = (*Provider)(nil)

// DefaultNetworkInstance is the OpenConfig network-instance name for the default VRF.
const DefaultNetworkInstance = "default"

func (p *Provider) EnsureBGP(ctx context.Context, req *provider.EnsureBGPRequest) error {
	if req.BGP.Spec.AdminState == v1alpha1.AdminStateDown {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.adminState",
			Description: "openconfig provider does not support disabling BGP (adminState Down)",
		})
	}

	spec := req.BGP.Spec

	asn, err := asnToUint32(spec.ASNumber)
	if err != nil {
		return err
	}

	ni := DefaultNetworkInstance
	if req.VRF != nil {
		ni = req.VRF.Spec.Name
	}

	var afiSafis *BGPAfiSafis
	if af := spec.AddressFamilies; af != nil {
		if af.L2vpnEvpn != nil {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.addressFamilies.l2vpnEvpn",
				Description: "openconfig provider does not support L2VPN EVPN address family",
			})
		}
		afiSafis = &BGPAfiSafis{}
		if af.Ipv4Unicast != nil {
			safi := &BGPAfiSafi{
				AfiSafiName: BGPAfiSafiTypeIPv4Unicast,
				Config: &BGPAfiSafiConfig{
					AfiSafiName: BGPAfiSafiTypeIPv4Unicast,
					Enabled:     af.Ipv4Unicast.Enabled,
				},
			}
			if mp := af.Ipv4Unicast.Multipath; mp != nil {
				ump := &BGPUseMultiplePaths{}
				if mp.Ebgp != nil {
					ump.Ebgp = &BGPEbgpMultipath{Config: &BGPEbgpMultipathConfig{
						AllowMultipleAS: mp.Ebgp.AllowMultipleAs,
						MaximumPaths:    uint32(mp.Ebgp.MaximumPaths), //nolint:gosec
					}}
				}
				if mp.Ibgp != nil {
					ump.Ibgp = &BGPIbgpMultipath{Config: &BGPIbgpMultipathConfig{
						MaximumPaths: uint32(mp.Ibgp.MaximumPaths), //nolint:gosec
					}}
				}
				safi.UseMultiplePaths = ump
			}
			afiSafis.AfiSafi.Set(safi)
		}
		if af.Ipv6Unicast != nil {
			safi := &BGPAfiSafi{
				AfiSafiName: BGPAfiSafiTypeIPv6Unicast,
				Config: &BGPAfiSafiConfig{
					AfiSafiName: BGPAfiSafiTypeIPv6Unicast,
					Enabled:     af.Ipv6Unicast.Enabled,
				},
			}
			if mp := af.Ipv6Unicast.Multipath; mp != nil {
				ump := &BGPUseMultiplePaths{}
				if mp.Ebgp != nil {
					ump.Ebgp = &BGPEbgpMultipath{Config: &BGPEbgpMultipathConfig{
						AllowMultipleAS: mp.Ebgp.AllowMultipleAs,
						MaximumPaths:    uint32(mp.Ebgp.MaximumPaths), //nolint:gosec
					}}
				}
				if mp.Ibgp != nil {
					ump.Ibgp = &BGPIbgpMultipath{Config: &BGPIbgpMultipathConfig{
						MaximumPaths: uint32(mp.Ibgp.MaximumPaths), //nolint:gosec
					}}
				}
				safi.UseMultiplePaths = ump
			}
			afiSafis.AfiSafi.Set(safi)
		}
	}

	proto := &BGPProtocol{
		NetworkInstance: ni,
		Config: &BGPProtocolConfig{
			Identifier: PolicyTypeBGP,
			Name:       "BGP",
		},
		BGP: &BGP{
			Global: &BGPGlobal{
				Config: &BGPGlobalConfig{
					AS:       asn,
					RouterID: spec.RouterID,
				},
				AfiSafis: afiSafis,
			},
		},
	}

	sb := new(gnmiext.SetBuilder)
	sb.Update(proto)

	// Handle redistribute direct routes via table-connections.
	for afType, policy := range req.RedistributeDirectRoutePolicies {
		af, err := toAddressFamily(afType)
		if err != nil {
			return err
		}
		tc := &TableConnection{
			NetworkInstance: ni,
			SrcProtocol:     PolicyTypeDirectlyConnected,
			DstProtocol:     PolicyTypeBGP,
			AddressFamily:   af,
			Config: &TableConnectionConfig{
				SrcProtocol:   PolicyTypeDirectlyConnected,
				DstProtocol:   PolicyTypeBGP,
				AddressFamily: af,
				ImportPolicy:  []string{policy.Spec.Name},
			},
		}
		sb.Update(tc)
	}

	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteBGP(ctx context.Context, req *provider.DeleteBGPRequest) error {
	ni := DefaultNetworkInstance
	if req.VRF != nil {
		ni = req.VRF.Spec.Name
	}

	proto := &BGPProtocol{NetworkInstance: ni}

	// Also clean up any table-connections for redistribution.
	tcIPv4 := &TableConnection{
		NetworkInstance: ni,
		SrcProtocol:     PolicyTypeDirectlyConnected,
		DstProtocol:     PolicyTypeBGP,
		AddressFamily:   AddressFamilyIPv4,
	}
	tcIPv6 := &TableConnection{
		NetworkInstance: ni,
		SrcProtocol:     PolicyTypeDirectlyConnected,
		DstProtocol:     PolicyTypeBGP,
		AddressFamily:   AddressFamilyIPv6,
	}

	return p.client.Delete(ctx, proto, tcIPv4, tcIPv6)
}

// toOCAddressFamily converts a BGPAddressFamilyType to the OpenConfig address-family identity.
func toAddressFamily(af v1alpha1.BGPAddressFamilyType) (AddressFamily, error) {
	switch af {
	case v1alpha1.BGPAddressFamilyIpv4Unicast:
		return AddressFamilyIPv4, nil
	case v1alpha1.BGPAddressFamilyIpv6Unicast:
		return AddressFamilyIPv6, nil
	default:
		return "", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.addressFamilies",
			Description: fmt.Sprintf("unsupported address family %q for table-connection", af),
		})
	}
}

// asnToUint32 converts an IntOrString ASN (plain or dotted notation) to a uint32.
func asnToUint32(asn intstr.IntOrString) (uint32, error) {
	if asn.Type == intstr.Int {
		return uint32(asn.IntVal), nil //nolint:gosec
	}
	s := asn.StrVal
	if !strings.Contains(s, ".") {
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid AS number %q: %w", s, err)
		}
		return uint32(v), nil
	}
	// Dotted notation: high.low → (high * 65536) + low
	parts := strings.SplitN(s, ".", 2)
	high, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid AS number %q: %w", s, err)
	}
	low, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid AS number %q: %w", s, err)
	}
	return uint32(high*65536) + uint32(low), nil
}

// PolicyType represents the OpenConfig policy-types protocol identity.
type PolicyType string

const (
	PolicyTypeBGP               PolicyType = "openconfig-policy-types:BGP"
	PolicyTypeISIS              PolicyType = "openconfig-policy-types:ISIS"
	PolicyTypeDirectlyConnected PolicyType = "openconfig-policy-types:DIRECTLY_CONNECTED"
)

// AddressFamily represents the OpenConfig address-family identity for table connections.
type AddressFamily string

const (
	AddressFamilyIPv4 AddressFamily = "openconfig-types:IPV4"
	AddressFamilyIPv6 AddressFamily = "openconfig-types:IPV6"
)

// BGPAfiSafiType represents the OpenConfig BGP AFI-SAFI type identity.
type BGPAfiSafiType string

const (
	BGPAfiSafiTypeIPv4Unicast BGPAfiSafiType = "openconfig-bgp-types:IPV4_UNICAST"
	BGPAfiSafiTypeIPv6Unicast BGPAfiSafiType = "openconfig-bgp-types:IPV6_UNICAST"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*BGPProtocol)(nil)
	_ gnmiext.DataElement = (*TableConnection)(nil)
)

// BGPProtocol represents the OC protocol[identifier=BGP] list entry.
type BGPProtocol struct {
	NetworkInstance string             `json:"-"`
	Config          *BGPProtocolConfig `json:"config,omitempty"`
	BGP             *BGP               `json:"bgp,omitempty"`
}

func (b *BGPProtocol) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]", b.NetworkInstance)
}

// BGPProtocolConfig holds the config for the protocol list entry.
type BGPProtocolConfig struct {
	Identifier PolicyType `json:"identifier"`
	Name       string     `json:"name"`
}

// BGP holds the bgp container.
type BGP struct {
	Global *BGPGlobal `json:"global,omitempty"`
}

// BGPGlobal holds the bgp/global container.
type BGPGlobal struct {
	Config   *BGPGlobalConfig `json:"config,omitempty"`
	AfiSafis *BGPAfiSafis     `json:"afi-safis,omitempty"`
}

// BGPGlobalConfig holds bgp/global/config.
type BGPGlobalConfig struct {
	AS       uint32 `json:"as"`
	RouterID string `json:"router-id"`
}

// BGPAfiSafis holds the afi-safis container.
type BGPAfiSafis struct {
	AfiSafi gnmiext.List[string, *BGPAfiSafi] `json:"afi-safi,omitempty"`
}

// BGPAfiSafi represents a single afi-safi list entry.
type BGPAfiSafi struct {
	AfiSafiName      BGPAfiSafiType       `json:"afi-safi-name"`
	Config           *BGPAfiSafiConfig    `json:"config,omitempty"`
	UseMultiplePaths *BGPUseMultiplePaths `json:"use-multiple-paths,omitempty"`
}

func (a *BGPAfiSafi) Key() string { return string(a.AfiSafiName) }

// BGPAfiSafiConfig holds afi-safi/config.
type BGPAfiSafiConfig struct {
	AfiSafiName BGPAfiSafiType `json:"afi-safi-name"`
	Enabled     bool           `json:"enabled"`
}

// BGPUseMultiplePaths holds the use-multiple-paths container.
type BGPUseMultiplePaths struct {
	Ebgp *BGPEbgpMultipath `json:"ebgp,omitempty"`
	Ibgp *BGPIbgpMultipath `json:"ibgp,omitempty"`
}

// BGPEbgpMultipath holds use-multiple-paths/ebgp.
type BGPEbgpMultipath struct {
	Config *BGPEbgpMultipathConfig `json:"config,omitempty"`
}

// BGPEbgpMultipathConfig holds use-multiple-paths/ebgp/config.
type BGPEbgpMultipathConfig struct {
	AllowMultipleAS bool   `json:"allow-multiple-as,omitempty"`
	MaximumPaths    uint32 `json:"maximum-paths,omitempty"`
}

// BGPIbgpMultipath holds use-multiple-paths/ibgp.
type BGPIbgpMultipath struct {
	Config *BGPIbgpMultipathConfig `json:"config,omitempty"`
}

// BGPIbgpMultipathConfig holds use-multiple-paths/ibgp/config.
type BGPIbgpMultipathConfig struct {
	MaximumPaths uint32 `json:"maximum-paths,omitempty"`
}

// TableConnection represents a table-connection list entry for route redistribution.
type TableConnection struct {
	NetworkInstance string                 `json:"-"`
	SrcProtocol     PolicyType             `json:"-"`
	DstProtocol     PolicyType             `json:"-"`
	AddressFamily   AddressFamily          `json:"-"`
	Config          *TableConnectionConfig `json:"config,omitempty"`
}

func (tc *TableConnection) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/table-connections/table-connection[src-protocol=%s][dst-protocol=%s][address-family=%s]", tc.NetworkInstance, tc.SrcProtocol, tc.DstProtocol, tc.AddressFamily)
}

// TableConnectionConfig holds table-connection/config.
type TableConnectionConfig struct {
	SrcProtocol   PolicyType    `json:"src-protocol"`
	DstProtocol   PolicyType    `json:"dst-protocol"`
	AddressFamily AddressFamily `json:"address-family"`
	ImportPolicy  []string      `json:"import-policy,omitempty"`
}
