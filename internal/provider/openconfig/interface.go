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

var _ provider.InterfaceProvider = (*Provider)(nil)

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	spec := req.Interface.Spec

	i := &Interface{
		Name: spec.Name,
		Config: &InterfaceConfig{
			Name:        spec.Name,
			Description: spec.Description,
			Enabled:     spec.AdminState == v1alpha1.AdminStateUp,
			MTU:         uint16(spec.MTU), //nolint:gosec
		},
	}

	switch spec.Type {
	case v1alpha1.InterfaceTypeLoopback:
		i.Config.Type = InterfaceTypeSoftwareLoopback
		i.Config.MTU = 0

	case v1alpha1.InterfaceTypePhysical:
		i.Config.Type = InterfaceTypeEthernetCsmacd
		if spec.Ethernet != nil && spec.Ethernet.FECMode != "" {
			fecMode, err := toEthernetFECMode(spec.Ethernet.FECMode)
			if err != nil {
				return err
			}
			i.Ethernet = &InterfaceEthernet{
				Config: &InterfaceEthernetConfig{
					FECMode: fecMode,
				},
			}
		}
		if req.AggregateParent != nil {
			if i.Ethernet == nil {
				i.Ethernet = &InterfaceEthernet{Config: &InterfaceEthernetConfig{}}
			}
			i.Ethernet.Config.AggregateID = req.AggregateParent.Spec.Name
		}

	case v1alpha1.InterfaceTypeAggregate:
		i.Config.Type = InterfaceTypeIEEE8023adLag
		i.Aggregation = &InterfaceAggregation{
			Config: &InterfaceAggregationConfig{
				LagType: "LACP",
			},
		}

	case v1alpha1.InterfaceTypeRoutedVLAN:
		i.Config.Type = InterfaceTypeL3IPVlan
		if req.VLAN != nil {
			i.RoutedVlan = &RoutedVlan{
				Config: &RoutedVlanConfig{
					Vlan: uint16(req.VLAN.Spec.ID), //nolint:gosec
				},
			}
		}

	case v1alpha1.InterfaceTypeSubinterface:
		return p.EnsureSubinterface(ctx, req)

	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.type",
			Description: fmt.Sprintf("unsupported interface type %q", spec.Type),
		})
	}

	if spec.Switchport != nil {
		i.Config.TPID = InterfaceTPIDDot1Q
		if err := i.SetSwitchport(spec.Switchport, spec.Type); err != nil {
			return err
		}
	}

	if req.IPv4 != nil {
		sub := &Subinterface{
			Index:  0,
			Config: &SubinterfaceConfig{Index: 0, Enabled: true},
		}

		switch v := req.IPv4.(type) {
		case provider.IPv4AddressList:
			addrs := &IPv4Addresses{}
			for i, prefix := range v {
				ip := prefix.Addr().String()
				addrType := IPv4AddressTypePrimary
				if i > 0 {
					addrType = IPv4AddressTypeSecondary
				}
				addrs.Address.Set(&IPv4Address{
					IP: ip,
					Config: &IPv4AddressConfig{
						IP:           ip,
						PrefixLength: uint8(prefix.Bits()), //nolint:gosec
						Type:         addrType,
					},
				})
			}
			sub.IPv4 = &InterfaceIPv4{
				Config:    &InterfaceIPv4Config{Enabled: true},
				Addresses: addrs,
			}

		case provider.IPv4Unnumbered:
			sub.IPv4 = &InterfaceIPv4{
				Config: &InterfaceIPv4Config{Enabled: true},
				Unnumbered: &IPv4Unnumbered{
					InterfaceRef: &UnnumberedInterfaceRef{
						Config: &UnnumberedInterfaceRefConfig{
							Interface: v.SourceInterface,
						},
					},
				},
			}
		}

		subs := &Subinterfaces{}
		subs.Subinterface.Set(sub)
		i.Subinterfaces = subs
	}

	return p.client.Update(ctx, i)
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	spec := req.Interface.Spec

	switch spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		i := &Interface{
			Name: spec.Name,
			Config: &InterfaceConfig{
				Name:    spec.Name,
				Type:    InterfaceTypeEthernetCsmacd,
				Enabled: false,
			},
		}
		return p.client.Update(ctx, i)

	case v1alpha1.InterfaceTypeLoopback, v1alpha1.InterfaceTypeAggregate, v1alpha1.InterfaceTypeRoutedVLAN:
		i := &Interface{Name: spec.Name}
		return p.client.Delete(ctx, i)

	case v1alpha1.InterfaceTypeSubinterface:
		if spec.Encapsulation == nil || spec.ParentInterfaceRef == nil {
			return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
				Field:       "spec.encapsulation",
				Description: "subinterface requires encapsulation and parentInterfaceRef",
			})
		}
		sub := &SubinterfaceEntry{
			ParentName: spec.ParentInterfaceRef.Name,
			Index:      uint32(spec.Encapsulation.Tag), //nolint:gosec
		}
		return p.client.Delete(ctx, sub)

	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.type",
			Description: fmt.Sprintf("unsupported interface type %q for delete", spec.Type),
		})
	}
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	state := &InterfaceOperState{Name: req.Interface.Spec.Name}
	if err := p.client.GetState(ctx, state); err != nil {
		return provider.InterfaceStatus{}, err
	}

	return provider.InterfaceStatus{
		OperStatus: state.OperStatus == InterfaceOperStatusUp,
	}, nil
}

func (p *Provider) InterfaceNameEqual(_ context.Context, a, b string) (bool, error) {
	return a == b, nil
}

func (p *Provider) EnsureSubinterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	spec := req.Interface.Spec
	if spec.Encapsulation == nil || spec.ParentInterfaceRef == nil {
		return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
			Field:       "spec.encapsulation",
			Description: "subinterface requires encapsulation and parentInterfaceRef",
		})
	}

	index := uint32(spec.Encapsulation.Tag) //nolint:gosec
	sub := &SubinterfaceEntry{
		ParentName: spec.ParentInterfaceRef.Name,
		Index:      index,
		Config:     &SubinterfaceConfig{Index: index, Enabled: spec.AdminState == v1alpha1.AdminStateUp},
	}

	switch spec.Encapsulation.Type {
	case v1alpha1.EncapsulationTypeDot1Q:
		sub.Vlan = &SubinterfaceVlan{
			Match: &SubinterfaceVlanMatch{
				SingleTagged: &SubinterfaceVlanSingleTagged{
					Config: &SubinterfaceVlanSingleTaggedConfig{
						VlanID: uint16(spec.Encapsulation.Tag), //nolint:gosec
					},
				},
			},
		}
	case v1alpha1.EncapsulationTypeQinQ:
		sub.Vlan = &SubinterfaceVlan{
			Match: &SubinterfaceVlanMatch{
				DoubleTagged: &SubinterfaceVlanDoubleTagged{
					Config: &SubinterfaceVlanDoubleTaggedConfig{
						InnerVlanID: uint16(spec.Encapsulation.InnerTag), //nolint:gosec
						OuterVlanID: uint16(spec.Encapsulation.OuterTag), //nolint:gosec
					},
				},
			},
		}
	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.encapsulation.type",
			Description: fmt.Sprintf("unsupported encapsulation type %q", spec.Encapsulation.Type),
		})
	}

	if req.IPv4 != nil {
		switch v := req.IPv4.(type) {
		case provider.IPv4AddressList:
			addrs := &IPv4Addresses{}
			for i, prefix := range v {
				ip := prefix.Addr().String()
				addrType := IPv4AddressTypePrimary
				if i > 0 {
					addrType = IPv4AddressTypeSecondary
				}
				addrs.Address.Set(&IPv4Address{
					IP: ip,
					Config: &IPv4AddressConfig{
						IP:           ip,
						PrefixLength: uint8(prefix.Bits()), //nolint:gosec
						Type:         addrType,
					},
				})
			}
			sub.IPv4 = &InterfaceIPv4{
				Config:    &InterfaceIPv4Config{Enabled: true},
				Addresses: addrs,
			}
		case provider.IPv4Unnumbered:
			sub.IPv4 = &InterfaceIPv4{
				Config: &InterfaceIPv4Config{Enabled: true},
				Unnumbered: &IPv4Unnumbered{
					InterfaceRef: &UnnumberedInterfaceRef{
						Config: &UnnumberedInterfaceRefConfig{
							Interface: v.SourceInterface,
						},
					},
				},
			}
		}
	}

	return p.client.Update(ctx, sub)
}

// InterfaceType represents the YANG identity for the interface type.
type InterfaceType string

const (
	InterfaceTypeEthernetCsmacd   InterfaceType = "iana-if-type:ethernetCsmacd"
	InterfaceTypeSoftwareLoopback InterfaceType = "iana-if-type:softwareLoopback"
	InterfaceTypeIEEE8023adLag    InterfaceType = "iana-if-type:ieee8023adLag"
	InterfaceTypeL3IPVlan         InterfaceType = "iana-if-type:l3ipvlan"
)

// EthernetFECMode represents the forward error correction mode.
type EthernetFECMode string

const (
	EthernetFECModeFC       EthernetFECMode = "FEC_FC"
	EthernetFECModeRS528    EthernetFECMode = "FEC_RS528"
	EthernetFECModeDisabled EthernetFECMode = "FEC_DISABLED"
)

func toEthernetFECMode(mode v1alpha1.FECMode) (EthernetFECMode, error) {
	switch mode {
	case v1alpha1.FECModeFC:
		return EthernetFECModeFC, nil
	case v1alpha1.FECModeRS528:
		return EthernetFECModeRS528, nil
	case v1alpha1.FECModeDisabled:
		return EthernetFECModeDisabled, nil
	default:
		return "", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.ethernet.fecMode",
			Description: fmt.Sprintf("unsupported FEC mode %q", mode),
		})
	}
}

// SwitchportMode represents the switchport interface mode.
type SwitchportMode string

const (
	SwitchportModeAccess SwitchportMode = "ACCESS"
	SwitchportModeTrunk  SwitchportMode = "TRUNK"
)

// InterfaceTPID represents the tag protocol identifier.
type InterfaceTPID string

const (
	InterfaceTPIDDot1Q InterfaceTPID = "TPID_0X8100"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*Interface)(nil)
	_ gnmiext.DataElement = (*InterfaceOperState)(nil)
	_ gnmiext.DataElement = (*SubinterfaceEntry)(nil)
)

// Interface represents an OpenConfig interface list entry.
type Interface struct {
	Name          string                `json:"-"`
	Config        *InterfaceConfig      `json:"config,omitempty"`
	Subinterfaces *Subinterfaces        `json:"subinterfaces,omitempty"`
	Ethernet      *InterfaceEthernet    `json:"openconfig-if-ethernet:ethernet,omitempty"`
	Aggregation   *InterfaceAggregation `json:"openconfig-if-aggregate:aggregation,omitempty"`
	RoutedVlan    *RoutedVlan           `json:"openconfig-vlan:routed-vlan,omitempty"`
}

func (i *Interface) XPath() string {
	return fmt.Sprintf("openconfig-interfaces:interfaces/interface[name=%s]", i.Name)
}

func (i *Interface) SetSwitchport(sp *v1alpha1.Switchport, ifType v1alpha1.InterfaceType) error {
	config := &SwitchedVlanConfig{}

	switch sp.Mode {
	case v1alpha1.SwitchportModeAccess:
		config.InterfaceMode = SwitchportModeAccess
		config.AccessVlan = uint16(sp.AccessVlan) //nolint:gosec
	case v1alpha1.SwitchportModeTrunk:
		config.InterfaceMode = SwitchportModeTrunk
		config.NativeVlan = uint16(sp.NativeVlan) //nolint:gosec
		for _, vlan := range sp.AllowedVlans {
			config.TrunkVlans = append(config.TrunkVlans, uint16(vlan)) //nolint:gosec
		}
	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.switchport.mode",
			Description: fmt.Sprintf("unsupported switchport mode %q", sp.Mode),
		})
	}

	switch ifType {
	case v1alpha1.InterfaceTypePhysical:
		if i.Ethernet == nil {
			i.Ethernet = &InterfaceEthernet{}
		}
		i.Ethernet.SwitchedVlan = &EthernetSwitchedVlan{Config: config}
	case v1alpha1.InterfaceTypeAggregate:
		if i.Aggregation == nil {
			i.Aggregation = &InterfaceAggregation{}
		}
		i.Aggregation.SwitchedVlan = &AggregationSwitchedVlan{Config: config}
	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.switchport",
			Description: fmt.Sprintf("switchport not supported on interface type %q", ifType),
		})
	}

	return nil
}

// InterfaceOperState retrieves the operational state of an interface.
type InterfaceOperState struct {
	Name       string              `json:"-"`
	OperStatus InterfaceOperStatus `json:"oper-status,omitempty"`
}

func (s *InterfaceOperState) XPath() string {
	return fmt.Sprintf("openconfig-interfaces:interfaces/interface[name=%s]/state", s.Name)
}

// InterfaceOperStatus represents the operational state of an interface.
type InterfaceOperStatus string

const (
	InterfaceOperStatusUp             InterfaceOperStatus = "UP"
	InterfaceOperStatusDown           InterfaceOperStatus = "DOWN"
	InterfaceOperStatusTesting        InterfaceOperStatus = "TESTING"
	InterfaceOperStatusUnknown        InterfaceOperStatus = "UNKNOWN"
	InterfaceOperStatusDormant        InterfaceOperStatus = "DORMANT"
	InterfaceOperStatusNotPresent     InterfaceOperStatus = "NOT_PRESENT"
	InterfaceOperStatusLowerLayerDown InterfaceOperStatus = "LOWER_LAYER_DOWN"
)

// InterfaceConfig holds the config container for an interface.
type InterfaceConfig struct {
	Name        string        `json:"name"`
	Type        InterfaceType `json:"type"`
	Description string        `json:"description,omitempty"`
	Enabled     bool          `json:"enabled"`
	MTU         uint16        `json:"mtu,omitempty"`
	TPID        InterfaceTPID `json:"tpid,omitempty"`
}

// Subinterfaces holds the subinterface list container.
type Subinterfaces struct {
	Subinterface gnmiext.List[uint32, *Subinterface] `json:"subinterface,omitempty"`
}

// Subinterface represents a single subinterface entry.
type Subinterface struct {
	Index  uint32              `json:"index"`
	Config *SubinterfaceConfig `json:"config,omitempty"`
	IPv4   *InterfaceIPv4      `json:"openconfig-if-ip:ipv4,omitempty"`
}

func (s *Subinterface) Key() uint32 {
	return s.Index
}

// SubinterfaceConfig holds the config container for a subinterface.
type SubinterfaceConfig struct {
	Index   uint32 `json:"index"`
	Enabled bool   `json:"enabled"`
}

// InterfaceIPv4 holds the IPv4 container for a subinterface.
type InterfaceIPv4 struct {
	Addresses  *IPv4Addresses       `json:"addresses,omitempty"`
	Unnumbered *IPv4Unnumbered      `json:"unnumbered,omitempty"`
	Config     *InterfaceIPv4Config `json:"config,omitempty"`
}

// InterfaceIPv4Config holds the config container for IPv4.
type InterfaceIPv4Config struct {
	Enabled bool `json:"enabled"`
}

// IPv4Addresses holds the IPv4 address list container.
type IPv4Addresses struct {
	Address gnmiext.List[string, *IPv4Address] `json:"address,omitempty"`
}

// IPv4Address represents a single IPv4 address entry.
type IPv4Address struct {
	IP     string             `json:"ip"`
	Config *IPv4AddressConfig `json:"config,omitempty"`
}

func (a *IPv4Address) Key() string {
	return a.IP
}

// IPv4AddressType represents the type of an IPv4 address.
type IPv4AddressType string

const (
	IPv4AddressTypePrimary   IPv4AddressType = "PRIMARY"
	IPv4AddressTypeSecondary IPv4AddressType = "SECONDARY"
)

// IPv4AddressConfig holds the config for a single IPv4 address.
type IPv4AddressConfig struct {
	IP           string          `json:"ip"`
	PrefixLength uint8           `json:"prefix-length"`
	Type         IPv4AddressType `json:"type,omitempty"`
}

// IPv4Unnumbered holds the unnumbered container for IPv4.
type IPv4Unnumbered struct {
	InterfaceRef *UnnumberedInterfaceRef `json:"interface-ref,omitempty"`
}

// UnnumberedInterfaceRef holds the interface-ref container.
type UnnumberedInterfaceRef struct {
	Config *UnnumberedInterfaceRefConfig `json:"config,omitempty"`
}

// UnnumberedInterfaceRefConfig holds the config for an unnumbered interface ref.
type UnnumberedInterfaceRefConfig struct {
	Interface string `json:"interface,omitempty"`
}

// InterfaceEthernet holds the openconfig-if-ethernet augmentation.
type InterfaceEthernet struct {
	Config       *InterfaceEthernetConfig `json:"config,omitempty"`
	SwitchedVlan *EthernetSwitchedVlan    `json:"openconfig-vlan:switched-vlan,omitempty"`
}

// InterfaceEthernetConfig holds the config container for Ethernet.
type InterfaceEthernetConfig struct {
	FECMode     EthernetFECMode `json:"fec-mode,omitempty"`
	AggregateID string          `json:"openconfig-if-aggregate:aggregate-id,omitempty"`
}

// EthernetSwitchedVlan holds the switched-vlan container for Ethernet.
type EthernetSwitchedVlan struct {
	Config *SwitchedVlanConfig `json:"config,omitempty"`
}

// SwitchedVlanConfig holds the config for switched-vlan.
type SwitchedVlanConfig struct {
	InterfaceMode SwitchportMode `json:"interface-mode,omitempty"`
	AccessVlan    uint16         `json:"access-vlan,omitempty"`
	NativeVlan    uint16         `json:"native-vlan,omitempty"`
	TrunkVlans    []uint16       `json:"trunk-vlans,omitempty"`
}

// InterfaceAggregation holds the openconfig-if-aggregate augmentation.
type InterfaceAggregation struct {
	Config       *InterfaceAggregationConfig `json:"config,omitempty"`
	SwitchedVlan *AggregationSwitchedVlan    `json:"openconfig-vlan:switched-vlan,omitempty"`
}

// InterfaceAggregationConfig holds the config container for aggregation.
type InterfaceAggregationConfig struct {
	LagType  string  `json:"lag-type,omitempty"`
	MinLinks *uint16 `json:"min-links,omitempty"`
}

// AggregationSwitchedVlan holds the switched-vlan container for aggregation.
type AggregationSwitchedVlan struct {
	Config *SwitchedVlanConfig `json:"config,omitempty"`
}

// RoutedVlan holds the routed-vlan container.
type RoutedVlan struct {
	Config *RoutedVlanConfig `json:"config,omitempty"`
}

// RoutedVlanConfig holds the config for routed-vlan.
type RoutedVlanConfig struct {
	Vlan uint16 `json:"vlan,omitempty"`
}

// SubinterfaceEntry targets a specific subinterface under a parent interface.
type SubinterfaceEntry struct {
	ParentName string              `json:"-"`
	Index      uint32              `json:"index"`
	Config     *SubinterfaceConfig `json:"config,omitempty"`
	IPv4       *InterfaceIPv4      `json:"openconfig-if-ip:ipv4,omitempty"`
	Vlan       *SubinterfaceVlan   `json:"openconfig-vlan:vlan,omitempty"`
}

func (s *SubinterfaceEntry) XPath() string {
	return fmt.Sprintf("openconfig-interfaces:interfaces/interface[name=%s]/subinterfaces/subinterface[index=%d]", s.ParentName, s.Index)
}

// SubinterfaceVlan holds the VLAN encapsulation for a subinterface.
type SubinterfaceVlan struct {
	Match *SubinterfaceVlanMatch `json:"match,omitempty"`
}

// SubinterfaceVlanMatch holds the VLAN match criteria.
type SubinterfaceVlanMatch struct {
	SingleTagged *SubinterfaceVlanSingleTagged `json:"single-tagged,omitempty"`
	DoubleTagged *SubinterfaceVlanDoubleTagged `json:"double-tagged,omitempty"`
}

// SubinterfaceVlanSingleTagged holds the single-tagged VLAN match config.
type SubinterfaceVlanSingleTagged struct {
	Config *SubinterfaceVlanSingleTaggedConfig `json:"config,omitempty"`
}

// SubinterfaceVlanSingleTaggedConfig holds the VLAN ID for single-tagged match.
type SubinterfaceVlanSingleTaggedConfig struct {
	VlanID uint16 `json:"vlan-id,omitempty"`
}

// SubinterfaceVlanDoubleTagged holds the double-tag match config.
type SubinterfaceVlanDoubleTagged struct {
	Config *SubinterfaceVlanDoubleTaggedConfig `json:"config,omitempty"`
}

// SubinterfaceVlanDoubleTaggedConfig holds inner/outer VLAN IDs for QinQ.
type SubinterfaceVlanDoubleTaggedConfig struct {
	InnerVlanID uint16 `json:"inner-vlan-id,omitempty"`
	OuterVlanID uint16 `json:"outer-vlan-id,omitempty"`
}
