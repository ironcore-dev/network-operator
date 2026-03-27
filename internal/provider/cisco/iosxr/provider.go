// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	cp "github.com/felix-kaestner/copy"
	"google.golang.org/grpc"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ provider.Provider          = &Provider{}
	_ provider.DeviceProvider    = &Provider{}
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

func (p *Provider) ListPorts(ctx context.Context) ([]provider.DevicePort, error) {
	iFaces := new(Ifaces)
	err := p.client.GetConfig(ctx, iFaces)
	if err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	dp := make([]provider.DevicePort, 0, len(iFaces.PhysIfList))
	for _, intf := range iFaces.PhysIfList {
		var speeds = []int32{}
		s, _ := ExtractInterfaceSpeedFromName(intf.Name)

		if n, err := MapInterfaceSpeedToNumeric(s); err == nil {
			speeds = append(speeds, n)
		}
		// Only return physical interfaces; ignore subinterfaces
		if s != "" {
			// (todo): name already contains the speed information, convert them to standardized string value (e.g. 10G, 25G, 40G, 100G)
			dp = append(dp, provider.DevicePort{
				ID:                  intf.Name,
				Type:                intf.Name,
				SupportedSpeedsGbps: speeds,
			})
		}

	}
	return dp, nil
}

func (p *Provider) GetDeviceInfo(ctx context.Context) (*provider.DeviceInfo, error) {
	i := new(BasicDeviceInfo)

	if err := p.client.GetState(ctx, i); err != nil {
		return nil, err
	}

	return &provider.DeviceInfo{
		Manufacturer:    Manufacturer,
		Model:           i.Model,
		SerialNumber:    i.SerialNumber,
		FirmwareVersion: i.FirmwareVersion,
	}, nil
}

func (p *Provider) Reboot(ctx context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support rebooting the device")
}

func (p *Provider) FactoryReset(ctx context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support factory reset")
}

func (p *Provider) Reprovision(cxt context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support reprovisioning")
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
	_, err := ExtractOwnerFromInterfaceName(name)
	if err != nil {
		return err
	}

	conf := make([]gnmiext.Configurable, 0, 2)

	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:

		iface := &Iface{}
		iface.Name = name
		iface.Description = req.Interface.Spec.Description

		// Check if interface is part of a bundle
		// Bundle configuration needs to happen in a separate gnmi call
		bundle_name := req.Interface.GetAnnotations()[v1alpha1.AggregateLabel]
		if bundle_name == "" {
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
		if bundle_name != "" {
			ifaceBundeConf := &Iface{}
			ifaceBundeConf.Name = name
			bundle_id, _, err := ExtractBundleIdAndVlanTagsFromName(bundle_name)
			if err != nil {
				return err
			}

			ifaceBundeConf.BundleMember = BundleMember{
				ID: BundleID{
					BundleID:     bundle_id,
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

		// Presence of an outerVlan Tag in the interface name indicates a subinterface
		// BE<id>.<VLAN_ID>
		_, outerVlan, err := ExtractBundleIdAndVlanTagsFromName(name)
		if err != nil {
			return err
		}

		if outerVlan != 0 {
			if req.Interface.Spec.Switchport != nil && outerVlan != req.Interface.Spec.Switchport.AccessVlan {
				message := fmt.Sprintf("AccessVlan must match bundle-ether name pattern BE<id>.<ACCESS_VLAN>. %d != %d",
					outerVlan, req.Interface.Spec.Switchport.AccessVlan)
				return errors.New(message)
			}

			// Unset for bundle subinterfaces
			iface.Mode = gnmiext.Empty(false)

			// make sure the parent bundle-ether interface bundle-ether<id> exits
			parentBunndle := strings.Split(name, ".")[0]
			tmp := cp.Deep(req.Interface)
			tmp.Spec.Name = parentBunndle
			bundle := NewBundleInterface(tmp)
			conf = append(conf, &bundle)

			iface.ModeNoPhysical = "default"
			iface.SubInterface = VlanSubInterface{
				VlanIdentifier: VlanIdentifier{
					FirstTag: outerVlan,
					VlanType: "vlan-type-dot1q",
				},
			}

			// Subinterface configures QAndQ vlan
			if req.Interface.Spec.Switchport != nil && req.Interface.Spec.Switchport.AccessVlan != 0 {
				iface.SubInterface.VlanIdentifier.SecondTag = req.Interface.Spec.Switchport.AccessVlan
				iface.SubInterface.VlanIdentifier.VlanType = "vlan-type-dot1ad"
			}
			conf = append(conf, &iface)
		} else {
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
		}
		return updateInterface(ctx, p.client, conf...)
	}
	return nil
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
	owner, err := ExtractOwnerFromInterfaceName(intName)
	if err != nil {
		message := "failed to extract MTU owner from interface name" + intName
		return MTUs{}, errors.New(message)
	}
	return MTUs{MTU: []MTU{{
		MTU:   mtu,
		Owner: string(owner),
	}}}, nil
}

func updateInterface(ctx context.Context, client gnmiext.Client, conf ...gnmiext.Configurable) error {
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

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
