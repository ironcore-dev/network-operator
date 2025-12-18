// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	cp "github.com/felix-kaestner/copy"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"

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
	p.conn, err = deviceutil.NewGrpcClient(ctx, conn)
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

	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		conf := make([]gnmiext.Configurable, 0, 2)

		iface := &PhysIf{}
		iface.Name = name
		iface.Description = req.Interface.Spec.Description

		//Check if interface is part of a bundle
		//Bundle configuration needs to happen in a sperate gnmi call
		bundle_name := req.Interface.GetAnnotations()[v1alpha1.AggregateLabel]
		if bundle_name == "" {
			iface.Statistics.LoadInterval = uint8(30)

			if req.Interface.Spec.MTU != 0 {
				mtu, err := NewMTU(name, req.Interface.Spec.MTU)
				if err != nil {
					return err
				}
				iface.MTUs = mtu
			}

			if !(req.Interface.Spec.IPv4 == nil) {
				//if len(req.Interface.Spec.IPv4.Addresses) == 0 {
				//	message := "no IPv4 address configured for interface " + name
				//	return errors.New(message)
				//}
				if len(req.Interface.Spec.IPv4.Addresses) > 1 {
					message := "multiple IPv4 addresses configured for interface " + name
					return errors.New(message)
				}

				// (fixme): support IPv6 addresses, IPv6 neighbor config
				ip := req.Interface.Spec.IPv4.Addresses[0].Addr().String()
				ipNet := req.Interface.Spec.IPv4.Addresses[0].Bits()

				iface.IPv4Network = IPv4Network{
					Addresses: AddressesIPv4{
						Primary: Primary{
							Address: ip,
							Netmask: strconv.Itoa(ipNet),
						},
					},
				}
			}
		}

		//Configure bundle member
		ifaceBundeConf := &PhysIf{}
		ifaceBundeConf.Name = name
		if bundle_name != "" {
			bundle_id, _ := ExtractBundleIdAndVlanTagsFromName(bundle_name)
			ifaceBundeConf.BundleMember = BundleMember{
				ID: BundleID{
					BundleID:    bundle_id,
					PortAcivity: string(PortActivityOn),
				},
			}
			conf = append(conf, ifaceBundeConf)
		}

		// (fixme): for the moment it is enought to keep this static
		// option1: extend existing interface spec
		// option2: create a custom iosxr config
		iface.Shutdown = gnmiext.Empty(false)
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateDown {
			iface.Shutdown = gnmiext.Empty(true)
		}
		conf = append(conf, iface)

		return updateInteface(ctx, p.client, conf...)

	case v1alpha1.InterfaceTypeAggregate:
		if err := CheckInterfaceNameTypeAggregate(name); err != nil {
			return err
		}

		//Presence of an outerVlan Tag indicates a subinterface
		//BE<id>.<VLAN_ID>
		_, outerVlan := ExtractBundleIdAndVlanTagsFromName(name)

		if outerVlan != req.Interface.Spec.Switchport.AccessVlan {
			message := fmt.Sprintf("AccesVlan must match bundle-ether name pattern BE<id>.<ACCESS_VLAN>. %d != %d",
				outerVlan, req.Interface.Spec.Switchport.AccessVlan)
			return errors.New(message)
		}

		iface := &BundleInterface{}
		iface.Name = name
		iface.Description = req.Interface.Spec.Description

		if outerVlan != 0 {
			iface.ModeNoPhysical = "default"

			iface.SubInterface = VlanSubInterface{
				VlanIdentifier: VlanIdentifier{
					FirstTag:  outerVlan,
					SecondTag: req.Interface.Spec.Switchport.AccessVlan,
					VlanType:  "vlan-type-dot1q",
				},
			}

			//Subinterface configures QAndQ vlan
			if req.Interface.Spec.Switchport.AccessVlan != 0 {
				iface.SubInterface.VlanIdentifier.SecondTag = req.Interface.Spec.Switchport.AccessVlan
				iface.SubInterface.VlanIdentifier.VlanType = "vlan-type-dot1ad"
			}

		} else {
			//Set Interface mode to virtual for bundle interfaces
			iface.Mode = gnmiext.Empty(true)

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

		}
		return updateInteface(ctx, p.client, iface)
	}
	return nil
}

func NewMTU(intName string, mtu int32) (MTUs, error) {
	owner, err := ExractMTUOwnerFromIfaceName(intName)
	if err != nil {
		message := "failed to extract MTU owner from interface name" + intName
		return MTUs{}, errors.New(message)
	}
	return MTUs{MTU: []MTU{{
		MTU:   mtu,
		Owner: string(owner),
	}}}, nil

}

func updateInteface(ctx context.Context, client gnmiext.Client, conf ...gnmiext.Configurable) error {
	for _, cf := range conf {
		// Check if an interface exists otherwise patch will fail
		got := cp.Deep(cf)
		err := client.GetConfig(ctx, got)
		if err != nil {
			// Interface does not exist, create it
			err = client.Create(ctx, cf)
			if err == nil {
				continue
			}
			return err

		}
		err = client.Patch(ctx, cf)
		if err != nil {
			return err
		}

	}
	return nil

}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	physif := &PhysIf{}
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

	providerStatus := provider.InterfaceStatus{
		OperStatus: true,
	}

	if state.State != string(StateUp) {
		providerStatus.OperStatus = false
	}

	return providerStatus, nil
}

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
