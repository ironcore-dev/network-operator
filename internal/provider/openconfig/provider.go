// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

type Provider struct{}

func (p *Provider) CreateInterface(ctx context.Context, iface *v1alpha1.Interface) error {
	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}
	d, err := deviceutil.GetDeviceFromMetadata(ctx, c, iface)
	if err != nil {
		return fmt.Errorf("failed to get device from metadata: %w", err)
	}
	conn, err := deviceutil.GetDeviceGrpcClient(ctx, c, d)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	defer conn.Close()
	client, err := ygnmi.NewClient(gpb.NewGNMIClient(conn), ygnmi.WithRequestLogLevel(6))
	if err != nil {
		return fmt.Errorf("failed to create ygnmi client: %w", err)
	}

	i := &Interface{Name: ygot.String(iface.Spec.Name)}
	switch iface.Spec.AdminState {
	case v1alpha1.AdminStateUp:
		i.Enabled = ygot.Bool(true)
	case v1alpha1.AdminStateDown:
		i.Enabled = ygot.Bool(false)
	default:
		return fmt.Errorf("invalid admin state: %s", iface.Spec.AdminState)
	}
	i.Description = ygot.String(iface.Spec.Description)
	switch iface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		i.Type = IETFInterfaces_InterfaceType_ethernetCsmacd
	case v1alpha1.InterfaceTypeLoopback:
		i.Type = IETFInterfaces_InterfaceType_softwareLoopback
	default:
		return fmt.Errorf("unsupported interface type: %s", iface.Spec.Type)
	}
	i.Mtu = ygot.Uint16(uint16(iface.Spec.MTU))
	for idx, addr := range iface.Spec.IPv4Addresses {
		switch {
		case addr == "":
			continue
		case len(addr) >= 10 && addr[:10] == "unnumbered":
			sourceIface := addr[11:] // Extract the source interface name
			i.GetOrCreateSubinterface(uint32(idx)).GetOrCreateIpv4().GetOrCreateUnnumbered().GetOrCreateInterfaceRef().SetInterface(sourceIface)
		default:
			var ip netip.Prefix
			if ip, err = netip.ParsePrefix(addr); err != nil {
				return fmt.Errorf("failed to parse IPv4 address %q: %w", addr, err)
			}
			i.GetOrCreateSubinterface(uint32(idx)).GetOrCreateIpv4().GetOrCreateAddress(ip.Addr().String()).SetPrefixLength(uint8(ip.Bits()))
		}
	}
	if iface.Spec.Switchport != nil {
		i.Tpid = VlanTypes_TPID_TYPES_TPID_0X8100
		port := i.GetOrCreateEthernet().GetOrCreateSwitchedVlan()
		switch iface.Spec.Switchport.Mode {
		case v1alpha1.SwitchportModeAccess:
			port.InterfaceMode = VlanTypes_VlanModeType_ACCESS
			port.AccessVlan = ygot.Uint16(uint16(iface.Spec.Switchport.AccessVlan))
		case v1alpha1.SwitchportModeTrunk:
			port.InterfaceMode = VlanTypes_VlanModeType_TRUNK
			port.NativeVlan = ygot.Uint16(uint16(iface.Spec.Switchport.NativeVlan))
			for _, vlan := range iface.Spec.Switchport.AllowedVlans {
				var union Interface_Ethernet_SwitchedVlan_TrunkVlans_Union
				if union, err = port.To_Interface_Ethernet_SwitchedVlan_TrunkVlans_Union(vlan); err != nil {
					return fmt.Errorf("failed to convert vlan %d to union type: %w", vlan, err)
				}
				port.TrunkVlans = append(port.TrunkVlans, union)
			}
		default:
			return fmt.Errorf("invalid switchport mode: %s", iface.Spec.Switchport.Mode)
		}
	}

	log := ctrl.LoggerFrom(ctx)
	b, err := ygot.Marshal7951(i)
	if err != nil {
		return fmt.Errorf("failed to marshal interface: %w", err)
	}
	log.V(1).Info("Marshalled interface", "interface", string(b))

	_, err = ygnmi.Update(ctx, client, Root().Interface(iface.Spec.Name).Config(), i, ygnmi.WithEncoding(gpb.Encoding_JSON), ygnmi.WithAppendModuleName(false))
	return err
}

func (p *Provider) DeleteInterface(ctx context.Context, iface *v1alpha1.Interface) error {
	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}
	d, err := deviceutil.GetDeviceFromMetadata(ctx, c, iface)
	if err != nil {
		return fmt.Errorf("failed to get device from metadata: %w", err)
	}
	conn, err := deviceutil.GetDeviceGrpcClient(ctx, c, d)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	defer conn.Close()
	client, err := ygnmi.NewClient(gpb.NewGNMIClient(conn), ygnmi.WithRequestLogLevel(6))
	if err != nil {
		return fmt.Errorf("failed to create ygnmi client: %w", err)
	}

	switch iface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		// For physical interfaces, we can't delete the interface directly.
		// Instead, we reset the configuration and set the admin state down.
		sb := new(ygnmi.SetBatch)
		ygnmi.BatchUpdate(sb, Root().Interface(iface.Spec.Name).Enabled().Config(), false)
		ygnmi.BatchDelete(sb, Root().Interface(iface.Spec.Name).Description().Config())
		ygnmi.BatchDelete(sb, Root().Interface(iface.Spec.Name).SubinterfaceMap().Config())
		ygnmi.BatchDelete(sb, Root().Interface(iface.Spec.Name).Ethernet().Config())
		ygnmi.BatchDelete(sb, Root().Interface(iface.Spec.Name).Ethernet().SwitchedVlan().Config())
		_, err = sb.Set(ctx, client, ygnmi.WithEncoding(gpb.Encoding_JSON), ygnmi.WithAppendModuleName(true))
		return err
	case v1alpha1.InterfaceTypeLoopback:
		_, err = ygnmi.Delete(ctx, client, Root().Interface(iface.Spec.Name).Config())
		return err
	}

	return fmt.Errorf("unsupported interface type: %s", iface.Spec.Type)
}

func (p *Provider) CreateDevice(ctx context.Context, _ *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "CreateDevice not implemented")
	return nil
}

func (p *Provider) DeleteDevice(ctx context.Context, _ *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "DeleteDevice not implemented")
	return nil
}

func init() {
	provider.Register("openconfig", &Provider{})
}
