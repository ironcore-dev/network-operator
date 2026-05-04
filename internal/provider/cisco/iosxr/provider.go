// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/transport/grpcext"

	"google.golang.org/grpc"
)

var (
	_ provider.Provider          = &Provider{}
	_ provider.InterfaceProvider = &Provider{}
)

type Provider struct {
	conn   *grpc.ClientConn
	client gnmiext.Client
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func (p *Provider) Connect(ctx context.Context, conn *deviceutil.Connection) (err error) {
	p.conn, err = grpcext.NewClient(conn)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	p.client, err = gnmiext.New(ctx, p.conn)
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) Disconnect(ctx context.Context, conn *deviceutil.Connection) error {
	return p.conn.Close()
}

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	if p.client == nil {
		return errors.New("client is not connected")
	}

	name := req.Interface.Spec.Name

	if err := ValidateInterfaceName(name); err != nil {
		return err
	}

	// Configure different interface types based on the interface name
	// Interface <PortSpeed><rack><slot><port> e.g TwentyFiveGigE0/0/0/3
	// SubInterface <PotySpeed><rack><slot><port>.<vlan-id> e.g TwentyFiveGigE0/0/0/3
	// Bundle Interface/Port Channel Bundle-Ether<BundleID>
	// Vlans over Bundle Bundle-Ether<BundleID>.<vlan-id>
	_, err := ExtractInterfaceSpeedFromName(name)
	if err != nil {
		return err
	}

	conf := make([]gnmiext.DataElement, 0, 2)

	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:

		iface := &Iface{}
		iface.Name = name
		iface.Description = req.Interface.Spec.Description

		// Check if interface is part of a bundle
		// Bundle configuration needs to happen in a separate gnmi call
		bundleName := req.Interface.GetLabels()[v1alpha1.AggregateLabel]
		if bundleName == "" {
			iface.Statistics.LoadInterval = uint8(30)

			vlan, err := ExtractVlanTagFromName(name)
			if err != nil {
				return err
			}

			// Configure Subinterface
			if vlan != 0 {
				iface.SubInterface = NewVlanSubinterface(vlan, 0, "vlan-type-dot1q")
				iface.ModeNoPhysical = "default"
			}

			if req.Interface.Spec.IPv4 != nil {
				if len(req.Interface.Spec.IPv4.Addresses) > 1 {
					message := "multiple IPv4 addresses configured for interface " + name
					return errors.New(message)
				}

				// (fixme): support IPv6 addresses, IPv6 neighbor config
				prefix := req.Interface.Spec.IPv4.Addresses[0]
				ip := prefix.Addr().String()
				netmask := net.IP(net.CIDRMask(prefix.Bits(), 32)).String()

				iface.IPv4Network = IPv4Network{
					Addresses: AddressesIPv4{
						Primary: Primary{
							Address: ip,
							Netmask: netmask,
						},
					},
				}
			}

			if req.Interface.Spec.MTU != 0 {
				mtu, err := NewMTU(name, req.Interface.Spec.MTU)
				if err != nil {
					return err
				}
				iface.MTUs = mtu
			}
		}

		// Make interface part of a bundle
		if bundleName != "" {
			ifaceBundeConf := &Iface{}
			ifaceBundeConf.Name = name
			bundleID, _, err := ExtractBundleAndSubinterfaceID(bundleName)
			if err != nil {
				return err
			}

			ifaceBundeConf.BundleMember = BundleMember{
				ID: BundleID{
					BundleID:     bundleID,
					PortActivity: string(PortActivityOn),
				},
			}
			iface = ifaceBundeConf
		}

		// (fixme): for the moment it is enough to keep this static
		// option1: extend existing interface spec
		// option2: create a custom iosxr config
		iface.Shutdown = gnmiext.Empty(false)
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateDown {
			iface.Shutdown = gnmiext.Empty(true)
		}
		conf = append(conf, iface)

		return updateInterface(ctx, p.client, conf...)
	case v1alpha1.InterfaceTypeAggregate:
		if err := CheckInterfaceNameTypeAggregate(name); err != nil {
			return err
		}

		iface := NewBundleInterface(req.Interface)

		bundleID, subinterfaceID, err := ExtractBundleAndSubinterfaceID(name)
		if err != nil {
			return err
		}

		if bundleID == 0 {
			return fmt.Errorf("failed to extract bundle ID from interface name %q", name)
		}

		// Bundle interface configuration
		if subinterfaceID == 0 {
			iface.Statistics.LoadInterval = uint8(30)

			mtu, err := NewMTU(name, req.Interface.Spec.MTU)
			if err != nil {
				return err
			}
			iface.MTUs = mtu

			iface.Bundle = Bundle{
				MinAct: MinimumActive{
					Links: 1,
				},
			}
			conf = append(conf, &iface)
		} else {
			// (fixme): introduce new interface type subresource first. comes via different PR
			return fmt.Errorf("subinterfaces for bundle interfaces are not supported yet: %q", name)

			// Bundle subinterface configuration
			// make sure the parent bundle-ether interface bundle-ether<id> exits
			// parentBunndle := strings.Split(name, ".")[0]
			// tmp := cp.Deep(req.Interface)
			// tmp.Spec.Name = parentBunndle
			// bundle := NewBundleInterface(tmp)
			// conf = append(conf, &bundle)

			// Unset for bundle subinterfaces
			// iface.Mode = gnmiext.Empty(false)
			// iface.ModeNoPhysical = "default"
			// iface.SubInterface = VlanSubInterface{
			// 	VlanIdentifier: VlanIdentifier{
			// 		FirstTag: req.Interface.Spec.Switchport.AccessVlan,
			// 		VlanType: "vlan-type-dot1q",
			// 	},
			// }

			// Subinterface configures QAndQ vlan
			// if req.Interface.Spec.Switchport.InnerVlan != 0 {
			//	iface.SubInterface.VlanIdentifier.SecondTag = req.Interface.Spec.Switchport.InnerVlan
			//	iface.SubInterface.VlanIdentifier.VlanType = "vlan-type-dot1ad"
			// }
			// conf = append(conf, &iface)
		}
		return updateInterface(ctx, p.client, conf...)
	case v1alpha1.InterfaceTypeLoopback, v1alpha1.InterfaceTypeRoutedVLAN, v1alpha1.InterfaceTypeSubinterface:
		return fmt.Errorf("interface type %q is currently not supported", req.Interface.Spec.Type)
	default:
		return fmt.Errorf("unexpected interface type %q", req.Interface.Spec.Type)
	}
}

func NewBundleInterface(req *v1alpha1.Interface) Iface {
	bundle := Iface{
		Name:        req.Spec.Name,
		Description: req.Spec.Description,
		// Set Interface mode to virtual for bundle interfaces
		Mode: gnmiext.Empty(true),
	}
	return bundle
}

func NewVlanSubinterface(firstTag, secondTag int32, vlanType string) VlanSubInterface {
	subInt := VlanSubInterface{}

	subInt.VlanIdentifier.FirstTag = firstTag
	subInt.VlanIdentifier.SecondTag = secondTag
	subInt.VlanIdentifier.VlanType = vlanType
	return subInt
}

func NewMTU(intName string, mtu int32) (MTUs, error) {
	owner, err := ExtractInterfaceSpeedFromName(intName)
	if err != nil {
		message := "failed to extract MTU owner from interface name" + intName
		return MTUs{}, errors.New(message)
	}
	return MTUs{MTU: []MTU{{
		MTU:   mtu,
		Owner: string(owner),
	}}}, nil
}

func updateInterface(ctx context.Context, client gnmiext.Client, conf ...gnmiext.DataElement) error {
	for _, cf := range conf {
		err := client.Update(ctx, cf)
		if err == nil {
			continue
		}
		return err
	}
	return nil
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	physif := &Iface{}
	physif.Name = req.Interface.Spec.Name

	if p.client == nil {
		return errors.New("client is not connected")
	}

	err := p.client.Delete(ctx, physif)
	if err != nil {
		return fmt.Errorf("failed to delete interface %s: %w", req.Interface.Spec.Name, err)
	}
	return nil
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	state := new(PhysIfState)
	state.Name = req.Interface.Spec.Name

	if p.client == nil {
		return provider.InterfaceStatus{}, errors.New("client is not connected")
	}

	err := p.client.GetState(ctx, state)
	if err != nil {
		return provider.InterfaceStatus{}, fmt.Errorf("failed to get interface status for %s: %w", req.Interface.Spec.Name, err)
	}

	return provider.InterfaceStatus{
		OperStatus: state.State == string(StateUp),
	}, nil
}

func (p *Provider) InterfaceNameEqual(_ context.Context, a, b string) (bool, error) {
	// TODO: implement provider specific logic to compare interface names
	return a == b, nil
}

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
