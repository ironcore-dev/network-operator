// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"net/netip"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var (
	_ provider.Provider          = &Provider{}
	_ provider.InterfaceProvider = &Provider{}
)

type Provider struct {
	conn   *grpc.ClientConn
	client *ygnmi.Client
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func (p *Provider) Connect(ctx context.Context, conn *deviceutil.Connection) (err error) {
	p.conn, err = deviceutil.NewGrpcClient(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	p.client, err = ygnmi.NewClient(gpb.NewGNMIClient(p.conn), ygnmi.WithRequestLogLevel(6))
	if err != nil {
		return fmt.Errorf("failed to create ygnmi client: %w", err)
	}
	return nil
}

func (p *Provider) Disconnect(context.Context, *deviceutil.Connection) error {
	return p.conn.Close()
}

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	log := ctrl.LoggerFrom(ctx)

	i := &Interface{Name: ygot.String(req.Interface.Spec.Name)}
	switch req.Interface.Spec.AdminState {
	case v1alpha1.AdminStateUp:
		i.Enabled = ygot.Bool(true)
	case v1alpha1.AdminStateDown:
		i.Enabled = ygot.Bool(false)
	default:
		return fmt.Errorf("invalid admin state: %s", req.Interface.Spec.AdminState)
	}
	i.Description = ygot.String(req.Interface.Spec.Description)
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		i.Type = IETFInterfaces_InterfaceType_ethernetCsmacd
	case v1alpha1.InterfaceTypeLoopback:
		i.Type = IETFInterfaces_InterfaceType_softwareLoopback
	default:
		return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}
	i.Mtu = ygot.Uint16(uint16(req.Interface.Spec.MTU))
	for idx, addr := range req.Interface.Spec.IPv4Addresses {
		switch {
		case addr == "":
			continue
		case len(addr) >= 10 && addr[:10] == "unnumbered":
			sourceIface := addr[11:] // Extract the source interface name
			i.GetOrCreateSubinterface(uint32(idx)).GetOrCreateIpv4().GetOrCreateUnnumbered().GetOrCreateInterfaceRef().SetInterface(sourceIface)
		default:
			ip, err := netip.ParsePrefix(addr)
			if err != nil {
				return fmt.Errorf("failed to parse IPv4 address %q: %w", addr, err)
			}
			i.GetOrCreateSubinterface(uint32(idx)).GetOrCreateIpv4().GetOrCreateAddress(ip.Addr().String()).SetPrefixLength(uint8(ip.Bits()))
		}
	}
	if req.Interface.Spec.Switchport != nil {
		i.Tpid = VlanTypes_TPID_TYPES_TPID_0X8100
		port := i.GetOrCreateEthernet().GetOrCreateSwitchedVlan()
		switch req.Interface.Spec.Switchport.Mode {
		case v1alpha1.SwitchportModeAccess:
			port.InterfaceMode = VlanTypes_VlanModeType_ACCESS
			port.AccessVlan = ygot.Uint16(uint16(req.Interface.Spec.Switchport.AccessVlan))
		case v1alpha1.SwitchportModeTrunk:
			port.InterfaceMode = VlanTypes_VlanModeType_TRUNK
			port.NativeVlan = ygot.Uint16(uint16(req.Interface.Spec.Switchport.NativeVlan))
			for _, vlan := range req.Interface.Spec.Switchport.AllowedVlans {
				union, err := port.To_Interface_Ethernet_SwitchedVlan_TrunkVlans_Union(vlan)
				if err != nil {
					return fmt.Errorf("failed to convert vlan %d to union type: %w", vlan, err)
				}
				port.TrunkVlans = append(port.TrunkVlans, union)
			}
		default:
			return fmt.Errorf("invalid switchport mode: %s", req.Interface.Spec.Switchport.Mode)
		}
	}

	b, err := ygot.Marshal7951(i)
	if err != nil {
		return fmt.Errorf("failed to marshal interface: %w", err)
	}
	log.V(1).Info("Marshalled interface", "interface", string(b))

	_, err = ygnmi.Update(ctx, p.client, Root().Interface(req.Interface.Spec.Name).Config(), i, ygnmi.WithEncoding(gpb.Encoding_JSON), ygnmi.WithSkipModuleNames())
	return err
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		// For physical interfaces, we can't delete the interface directly.
		// Instead, we reset the configuration and set the admin state down.
		sb := new(ygnmi.SetBatch)
		ygnmi.BatchUpdate(sb, Root().Interface(req.Interface.Spec.Name).Enabled().Config(), false)
		ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Description().Config())
		ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).SubinterfaceMap().Config())
		ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Ethernet().Config())
		ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Ethernet().SwitchedVlan().Config())
		_, err := sb.Set(ctx, p.client, ygnmi.WithEncoding(gpb.Encoding_JSON), ygnmi.WithSkipModuleNames())
		return err
	case v1alpha1.InterfaceTypeLoopback:
		_, err := ygnmi.Delete(ctx, p.client, Root().Interface(req.Interface.Spec.Name).Config())
		return err
	}

	return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
}

func (p *Provider) GetInterfaceStatus(context.Context, *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	return provider.InterfaceStatus{}, nil
}

func init() {
	provider.Register("openconfig", NewProvider)
}
