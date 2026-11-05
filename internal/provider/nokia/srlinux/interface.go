// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.InterfaceProvider = (*Provider)(nil)

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	spec := req.Interface.Spec

	// RoutedVLAN (SVI) and switchport are realized through the SR Linux
	// mac-vrf/network-instance, which is owned by a VLANProvider the srlinux
	// provider does not implement yet. Reject them explicitly rather than write
	// a half-configured interface.
	if spec.Type == v1alpha1.InterfaceTypeRoutedVLAN {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       fieldSpecType,
			Description: "srlinux provider does not yet support RoutedVLAN interfaces (requires mac-vrf/network-instance support)",
		})
	}
	if spec.Switchport != nil {
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.switchport",
			Description: "srlinux provider does not yet support switchport configuration (requires mac-vrf/network-instance support)",
		})
	}

	if spec.Type == v1alpha1.InterfaceTypeSubinterface {
		return p.ensureSubinterface(ctx, req)
	}

	iface := &ifaceConfig{
		Name:        spec.Name,
		AdminState:  toAdminState(spec.AdminState),
		Description: spec.Description,
	}

	// routed marks the interface's own subinterface 0 as routed; loopbacks have no
	// interface type on SR Linux.
	routed := true

	switch spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		if spec.MTU > 0 {
			iface.MTU = uint16(spec.MTU) //nolint:gosec
		}
		if spec.Ethernet != nil && spec.Ethernet.FECMode != "" {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.ethernet.fecMode",
				Description: "srlinux provider does not support explicit forward error correction configuration",
			})
		}
		// A physical port enslaved to a LAG carries only its aggregate-id; its
		// layer-3 config lives on the aggregate interface.
		if req.AggregateParent != nil {
			iface.Ethernet = &ethernetConfig{AggregateID: req.AggregateParent.Spec.Name}
		}

	case v1alpha1.InterfaceTypeLoopback:
		routed = false

	case v1alpha1.InterfaceTypeAggregate:
		if spec.MTU > 0 {
			iface.MTU = uint16(spec.MTU) //nolint:gosec
		}
		iface.LAG = buildLAG(spec, req)

	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       fieldSpecType,
			Description: fmt.Sprintf("unsupported interface type %q", spec.Type),
		})
	}

	// Merge the interface-level leaves so sibling subinterfaces (owned by separate
	// Subinterface resources) are not wiped, then write this interface's own
	// subinterface 0 at its sub-path when it carries IP configuration.
	sb := new(gnmiext.SetBuilder)
	sb.Patch(iface)
	if sub := ipSubinterface(spec.Name, 0, routed, req); sub != nil {
		sb.Update(sub)
	}
	if spec.BFD != nil {
		sb.Update(bfdFromSpec(spec.Name+".0", spec.BFD))
	}
	return p.client.Do(ctx, sb)
}

func (p *Provider) ensureSubinterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	spec := req.Interface.Spec
	if spec.Encapsulation == nil || spec.ParentInterfaceRef == nil {
		return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
			Field:       "spec.encapsulation",
			Description: "subinterface requires encapsulation and parentInterfaceRef",
		})
	}

	parent, index, err := parseSubinterfaceName(spec.Name)
	if err != nil {
		return err
	}
	vlan, err := encapToVLAN(spec.Encapsulation)
	if err != nil {
		return err
	}

	sub := &subinterface{
		ParentName: parent,
		Index:      index,
		AdminState: toAdminState(spec.AdminState),
		Type:       subIfTypeRouted,
		VLAN:       vlan,
	}
	if spec.MTU > 0 {
		sub.IPMTU = uint16(spec.MTU) //nolint:gosec
	}
	sub.IPv4 = buildIPv4(req.IPv4)
	sub.IPv6 = buildIPv6(req.IPv6)

	// Tagged subinterfaces require vlan-tagging on the parent; patch it in without
	// replacing the parent's other configuration.
	sb := new(gnmiext.SetBuilder)
	sb.Patch(&vlanTagging{Name: parent, Value: true})
	sb.Update(sub)
	if spec.BFD != nil {
		sb.Update(bfdFromSpec(fmt.Sprintf("%s.%d", parent, index), spec.BFD))
	}
	return p.client.Do(ctx, sb)
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.DeleteInterfaceRequest) error {
	spec := req.Interface.Spec

	switch spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		// A physical port cannot be removed; reset it to administratively down and
		// clear any layer-3 config by replacing it with a bare disabled interface.
		sb := new(gnmiext.SetBuilder)
		sb.Update(&ifaceConfig{Name: spec.Name, AdminState: adminDisable})
		sb.Delete(&bfdSubinterface{ID: spec.Name + ".0"})
		return p.client.Do(ctx, sb)

	case v1alpha1.InterfaceTypeLoopback, v1alpha1.InterfaceTypeAggregate:
		sb := new(gnmiext.SetBuilder)
		sb.Delete(&ifaceConfig{Name: spec.Name})
		sb.Delete(&bfdSubinterface{ID: spec.Name + ".0"})
		return p.client.Do(ctx, sb)

	case v1alpha1.InterfaceTypeSubinterface:
		if spec.Encapsulation == nil || spec.ParentInterfaceRef == nil {
			return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
				Field:       "spec.encapsulation",
				Description: "subinterface requires encapsulation and parentInterfaceRef",
			})
		}
		parent, index, err := parseSubinterfaceName(spec.Name)
		if err != nil {
			return err
		}
		sb := new(gnmiext.SetBuilder)
		sb.Delete(&subinterface{ParentName: parent, Index: index})
		sb.Delete(&bfdSubinterface{ID: fmt.Sprintf("%s.%d", parent, index)})
		return p.client.Do(ctx, sb)

	case v1alpha1.InterfaceTypeRoutedVLAN:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       fieldSpecType,
			Description: "srlinux provider does not yet support RoutedVLAN interfaces (requires mac-vrf/network-instance support)",
		})

	default:
		return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       fieldSpecType,
			Description: fmt.Sprintf("unsupported interface type %q for delete", spec.Type),
		})
	}
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	name := req.Interface.Spec.Name
	if req.Interface.Spec.Type == v1alpha1.InterfaceTypeSubinterface && req.Interface.Spec.ParentInterfaceRef != nil {
		name = req.Interface.Spec.ParentInterfaceRef.Name
	}
	state := &ifaceOperState{Name: name}
	if err := p.client.GetState(ctx, state); err != nil {
		if errors.Is(err, gnmiext.ErrNil) {
			return provider.InterfaceStatus{}, nil
		}
		return provider.InterfaceStatus{}, err
	}
	return provider.InterfaceStatus{OperStatus: state.OperState == operStateUp}, nil
}

func (*Provider) InterfaceNameEqual(_ context.Context, a, b string) (bool, error) {
	return a == b, nil
}

func (*Provider) LoopbackInterfaceName(id int) (string, error) {
	return fmt.Sprintf("lo%d", id), nil
}

// toAdminState maps the API admin state to the SR Linux enable/disable leaf.
func toAdminState(s v1alpha1.AdminState) string {
	if s == v1alpha1.AdminStateDown {
		return adminDisable
	}
	return adminEnable
}

// ipSubinterface builds subinterface index 0 for the named interface carrying the
// requested IP configuration, or nil if none was requested. routed marks it as a
// routed subinterface (not applicable to loopbacks, where SR Linux has no interface
// type).
func ipSubinterface(name string, index uint32, routed bool, req *provider.EnsureInterfaceRequest) *subinterface {
	// A port enslaved to a LAG carries no layer-3 config of its own.
	if req.AggregateParent != nil {
		return nil
	}
	v4 := buildIPv4(req.IPv4)
	v6 := buildIPv6(req.IPv6)
	if v4 == nil && v6 == nil {
		return nil
	}
	sub := &subinterface{
		ParentName: name,
		Index:      index,
		AdminState: adminEnable,
		IPv4:       v4,
		IPv6:       v6,
	}
	if routed {
		sub.Type = subIfTypeRouted
	}
	return sub
}

// buildIPv4 builds the ipv4 container from the resolved request, or nil if no
// IPv4 configuration was requested.
func buildIPv4(ip provider.IPv4) *subIPv4 {
	switch v := ip.(type) {
	case provider.IPv4AddressList:
		out := &subIPv4{AdminState: adminEnable}
		for _, prefix := range v {
			out.Address.Set(&ipAddress{IPPrefix: prefix.String()})
		}
		return out
	case provider.IPv4Unnumbered:
		return &subIPv4{
			AdminState: adminEnable,
			Unnumbered: &unnumbered{AdminState: adminEnable, Interface: v.SourceInterface},
		}
	default:
		return nil
	}
}

// buildIPv6 builds the ipv6 container. A link-local-only request enables IPv6
// without a global address; SR Linux auto-derives the link-local address.
func buildIPv6(ip provider.IPv6) *subIPv6 {
	switch v := ip.(type) {
	case provider.IPv6AddressList:
		out := &subIPv6{AdminState: adminEnable}
		for _, prefix := range v {
			out.Address.Set(&ipAddress{IPPrefix: prefix.String()})
		}
		return out
	case provider.IPv6LinkLocalOnly:
		return &subIPv6{AdminState: adminEnable}
	default:
		return nil
	}
}

// buildLAG maps the aggregation spec to the SR Linux lag container.
func buildLAG(spec v1alpha1.InterfaceSpec, req *provider.EnsureInterfaceRequest) *lagConfig {
	lag := &lagConfig{}
	if spec.Aggregation.ControlProtocol.Mode == v1alpha1.LACPModeStatic {
		lag.LagType = lagTypeStatic
		return lag
	}

	lag.LagType = lagTypeLACP
	lacp := &lacpConfig{LACPMode: lacpModeActive}
	if spec.Aggregation.ControlProtocol.Mode == v1alpha1.LACPModePassive {
		lacp.LACPMode = lacpModePassive
	}
	// Multi-chassis LAG shares a LACP admin-key across both chassis.
	if req.MultiChassisID != nil {
		lacp.AdminKey = uint16(*req.MultiChassisID) //nolint:gosec
	}
	lag.LACP = lacp
	return lag
}

// encapToVLAN derives the subinterface vlan encapsulation from the encapsulation spec.
func encapToVLAN(enc *v1alpha1.Encapsulation) (*subVLAN, error) {
	switch enc.Type {
	case v1alpha1.EncapsulationTypeDot1Q:
		return &subVLAN{Encap: &vlanEncap{
			SingleTagged: &singleTagged{VLANID: uint16(enc.Tag)}, //nolint:gosec
		}}, nil
	case v1alpha1.EncapsulationTypeQinQ:
		return &subVLAN{Encap: &vlanEncap{
			DoubleTagged: &doubleTagged{
				OuterVLANID: uint16(enc.OuterTag), //nolint:gosec
				InnerVLANID: uint16(enc.InnerTag), //nolint:gosec
			},
		}}, nil
	default:
		return nil, apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.encapsulation.type",
			Description: fmt.Sprintf("unsupported encapsulation type %q", enc.Type),
		})
	}
}

// parseSubinterfaceName splits a canonical SR Linux subinterface name
// ("<parent>.<index>", e.g. "ethernet-1/2.100") into its parent interface and
// numeric index.
func parseSubinterfaceName(name string) (string, uint32, error) {
	parent, idx, ok := strings.Cut(name, ".")
	if !ok {
		return "", 0, apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
			Field:       "spec.name",
			Description: fmt.Sprintf("subinterface name %q must be of the form <parent>.<index>", name),
		})
	}
	index, err := strconv.ParseUint(idx, 10, 32)
	if err != nil {
		return "", 0, apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
			Field:       "spec.name",
			Description: fmt.Sprintf("subinterface name %q has a non-numeric index", name),
		})
	}
	return parent, uint32(index), nil
}

// bfdFromSpec maps the BFD spec to a bfd subinterface entry for the given id
// ("<interface>.<index>"). Intervals are converted to microseconds.
func bfdFromSpec(id string, bfd *v1alpha1.BFD) *bfdSubinterface {
	out := &bfdSubinterface{ID: id, AdminState: adminDisable}
	if bfd.Enabled {
		out.AdminState = adminEnable
	}
	if bfd.DesiredMinimumTxInterval != nil {
		out.DesiredMinTxInterval = uint32(bfd.DesiredMinimumTxInterval.Microseconds()) //nolint:gosec
	}
	if bfd.RequiredMinimumReceive != nil {
		out.RequiredMinRx = uint32(bfd.RequiredMinimumReceive.Microseconds()) //nolint:gosec
	}
	if bfd.DetectionMultiplier != nil {
		out.DetectionMultiplier = uint8(*bfd.DetectionMultiplier) //nolint:gosec
	}
	return out
}
