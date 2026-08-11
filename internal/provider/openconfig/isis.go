// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.ISISProvider = (*Provider)(nil)

func (p *Provider) EnsureISIS(ctx context.Context, req *provider.EnsureISISRequest) error {
	spec := req.ISIS.Spec

	ni := DefaultNetworkInstance

	levelCap, err := toISISLevelCapability(spec.Type)
	if err != nil {
		return err
	}

	proto := &ISISProtocol{
		NetworkInstance: ni,
		Name:            spec.Instance,
		Config: &ISISProtocolConfig{
			Identifier: PolicyTypeISIS,
			Name:       spec.Instance,
			Enabled:    spec.AdminState == v1alpha1.AdminStateUp,
		},
		ISIS: &ISIS{
			Global: &ISISGlobal{
				Config: &ISISGlobalConfig{
					LevelCapability: levelCap,
					Net:             []string{spec.NetworkEntityTitle},
				},
			},
		},
	}

	if len(spec.AddressFamilies) > 0 {
		proto.ISIS.Global.AfiSafi = &ISISAfiSafi{}
		for _, af := range spec.AddressFamilies {
			afi, safi := toISISAfiSafi(af)
			proto.ISIS.Global.AfiSafi.AF.Set(&ISISAf{
				AfiName:  afi,
				SafiName: safi,
				Config: &ISISAfConfig{
					AfiName:  afi,
					SafiName: safi,
					Enabled:  true,
				},
			})
		}
	}

	if spec.OverloadBit == v1alpha1.OverloadBitAlways {
		proto.ISIS.Global.LspBit = &ISISLspBit{
			OverloadBit: &ISISOverloadBit{
				Config: &ISISOverloadBitConfig{SetBit: true},
			},
		}
	}

	sb := new(gnmiext.SetBuilder)
	sb.Update(proto)

	for _, iface := range req.Interfaces {
		sb.Update(&ISISInterface{
			NetworkInstance: ni,
			ProtocolName:    spec.Instance,
			InterfaceID:     iface.Spec.Name,
			Config: &ISISInterfaceConfig{
				InterfaceID: iface.Spec.Name,
				Enabled:     true,
			},
		})
	}

	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteISIS(ctx context.Context, req *provider.DeleteISISRequest) error {
	ni := DefaultNetworkInstance
	proto := &ISISProtocol{
		NetworkInstance: ni,
		Name:            req.ISIS.Spec.Instance,
	}
	return p.client.Delete(ctx, proto)
}

func toISISLevelCapability(level v1alpha1.ISISLevel) (ISISLevelCapability, error) {
	switch level {
	case v1alpha1.ISISLevel1:
		return ISISLevelCapabilityLevel1, nil
	case v1alpha1.ISISLevel2:
		return ISISLevelCapabilityLevel2, nil
	case v1alpha1.ISISLevel12:
		return ISISLevelCapabilityLevel1_2, nil
	default:
		return "", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.type",
			Description: fmt.Sprintf("unsupported ISIS level %q", level),
		})
	}
}

func toISISAfiSafi(af v1alpha1.AddressFamily) (ISISAfiName, ISISSafiName) { //nolint:unparam
	switch af {
	case v1alpha1.AddressFamilyIPv6Unicast:
		return ISISAfiNameIPv6, ISISSafiNameUnicast
	default: // IPv4Unicast
		return ISISAfiNameIPv4, ISISSafiNameUnicast
	}
}

// ISISLevelCapability represents the ISIS level capability identity.
type ISISLevelCapability string

const (
	ISISLevelCapabilityLevel1   ISISLevelCapability = "LEVEL_1"
	ISISLevelCapabilityLevel2   ISISLevelCapability = "LEVEL_2"
	ISISLevelCapabilityLevel1_2 ISISLevelCapability = "LEVEL_1_2"
)

// ISISAfiName represents the ISIS address family identifier.
type ISISAfiName string

const (
	ISISAfiNameIPv4 ISISAfiName = "openconfig-isis-types:IPV4"
	ISISAfiNameIPv6 ISISAfiName = "openconfig-isis-types:IPV6"
)

// ISISSafiName represents the ISIS sub-address family identifier.
type ISISSafiName string

const (
	ISISSafiNameUnicast ISISSafiName = "openconfig-isis-types:UNICAST"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*ISISProtocol)(nil)
	_ gnmiext.DataElement = (*ISISInterface)(nil)
)

// ISISProtocol represents the OC protocol[identifier=ISIS] entry.
type ISISProtocol struct {
	NetworkInstance string              `json:"-"`
	Name            string              `json:"-"`
	Config          *ISISProtocolConfig `json:"config,omitempty"`
	ISIS            *ISIS               `json:"isis,omitempty"`
}

func (i *ISISProtocol) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=%s]", i.NetworkInstance, i.Name)
}

// ISISProtocolConfig holds the protocol config.
type ISISProtocolConfig struct {
	Identifier PolicyType `json:"identifier"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
}

// ISIS holds the isis container.
type ISIS struct {
	Global *ISISGlobal `json:"global,omitempty"`
}

// ISISGlobal holds isis/global.
type ISISGlobal struct {
	Config  *ISISGlobalConfig `json:"config,omitempty"`
	AfiSafi *ISISAfiSafi      `json:"afi-safi,omitempty"`
	LspBit  *ISISLspBit       `json:"lsp-bit,omitempty"`
}

// ISISGlobalConfig holds isis/global/config.
type ISISGlobalConfig struct {
	LevelCapability ISISLevelCapability `json:"level-capability"`
	Net             []string            `json:"net"`
}

// ISISAfiSafi holds isis/global/afi-safi.
type ISISAfiSafi struct {
	AF gnmiext.List[string, *ISISAf] `json:"af,omitempty"`
}

// ISISAf represents an ISIS afi-safi entry.
type ISISAf struct {
	AfiName  ISISAfiName   `json:"afi-name"`
	SafiName ISISSafiName  `json:"safi-name"`
	Config   *ISISAfConfig `json:"config,omitempty"`
}

func (a *ISISAf) Key() string { return string(a.AfiName) + "/" + string(a.SafiName) }

// ISISAfConfig holds isis af config.
type ISISAfConfig struct {
	AfiName  ISISAfiName  `json:"afi-name"`
	SafiName ISISSafiName `json:"safi-name"`
	Enabled  bool         `json:"enabled"`
}

// ISISLspBit holds isis/global/lsp-bit.
type ISISLspBit struct {
	OverloadBit *ISISOverloadBit `json:"overload-bit,omitempty"`
}

// ISISOverloadBit holds overload-bit container.
type ISISOverloadBit struct {
	Config *ISISOverloadBitConfig `json:"config,omitempty"`
}

// ISISOverloadBitConfig holds overload-bit/config.
type ISISOverloadBitConfig struct {
	SetBit bool `json:"set-bit"`
}

// ISISInterface targets an ISIS interface config.
type ISISInterface struct {
	NetworkInstance string               `json:"-"`
	ProtocolName    string               `json:"-"`
	InterfaceID     string               `json:"-"`
	Config          *ISISInterfaceConfig `json:"config,omitempty"`
}

func (i *ISISInterface) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=%s]/isis/interfaces/interface[interface-id=%s]", i.NetworkInstance, i.ProtocolName, i.InterfaceID)
}

// ISISInterfaceConfig holds the per-interface config.
type ISISInterfaceConfig struct {
	InterfaceID string `json:"interface-id"`
	Enabled     bool   `json:"enabled"`
}
