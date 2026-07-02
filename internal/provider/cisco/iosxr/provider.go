// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/transport/grpcext"

	"google.golang.org/grpc"
)

var (
	_ provider.Provider          = &Provider{}
	_ provider.DeviceProvider    = &Provider{}
	_ provider.InterfaceProvider = &Provider{}
	_ provider.VRFProvider       = &Provider{}
	_ provider.BGPProvider       = &Provider{}
	_ provider.BGPPeerProvider   = &Provider{}
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

func (p *Provider) ListPorts(ctx context.Context) ([]provider.DevicePort, error) {
	iFaces := new(Ifaces)
	err := p.client.GetConfig(ctx, iFaces)
	if err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	dp := make([]provider.DevicePort, 0, len(iFaces.PhysIfList))
	for _, intf := range iFaces.PhysIfList {
		var s IFaceOwner
		var n int32
		if s, err = ExtractOwnerFromInterfaceName(intf.Name); err != nil {
			return nil, fmt.Errorf("failed to map interface speed to numeric value: %w", err)
		}

		if n, err = MapInterfaceOwnerToSpeed(s); err != nil {
			return nil, fmt.Errorf("failed to map interface speed to numeric value: %w", err)
		}

		dp = append(dp, provider.DevicePort{
			ID:                  intf.Name,
			Type:                string(s),
			SupportedSpeedsGbps: []int32{n},
		})
	}
	return dp, nil
}

func (p *Provider) GetDeviceInfo(ctx context.Context) (*provider.DeviceInfo, error) {
	i := new(BasicDeviceInfo)
	hostName := new(Hostname)

	if err := p.client.GetConfig(ctx, hostName); err != nil {
		return nil, err
	}

	if err := p.client.GetState(ctx, i); err != nil {
		return nil, err
	}

	return &provider.DeviceInfo{
		Manufacturer:    Manufacturer,
		Model:           i.Model,
		SerialNumber:    i.SerialNumber,
		FirmwareVersion: i.FirmwareVersion,
		Hostname:        string(*hostName),
	}, nil
}

func (p *Provider) GetLastRebootTime(ctx context.Context) (time.Time, error) {
	t := new(SystemTime)
	if err := p.client.GetState(ctx, t); err != nil {
		return time.Time{}, err
	}
	uptimeDuration := time.Second * time.Duration(t.Uptime.Uptime) * -1
	return t.CurrTime.ConvertToTime().Add(uptimeDuration), nil
}

func (p *Provider) Reboot(_ context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support rebooting the device")
}

func (p *Provider) FactoryReset(_ context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support factory reset")
}

func (p *Provider) Reprovision(_ context.Context, conn *deviceutil.Connection) error {
	return errors.New("IOS XR Provider does not support reprovisioning")
}

// EnsureInterface configures the interface based on the provided request.
// MTU configuration rules:
//   - Physical interface:
//     Configure L2 MTU and, if an IP address is present, configure L3 MTU.
//   - Bundle interface:
//     Configure only the L2 MTU.
//   - Bundle member interface (Physical):
//     Do not configure MTU settings directly.
//     L2 MTU is inherited from the bundle interface, and L3 MTU is configured
//     on the corresponding subinterface.
//   - Subinterface (physical or bundle):
//     Configure only the L3 MTU, using a default value of 1500 bytes.
func (p *Provider) EnsureInterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	name := req.Interface.Spec.Name

	if err := ValidateInterfaceName(name); err != nil {
		return err
	}

	// Configure different interface types based on the interface name
	// Interface <PortSpeed><rack><slot><port> e.g TwentyFiveGigE0/0/0/3
	// SubInterface <PotySpeed><rack><slot><port>.<vlan-id> e.g TwentyFiveGigE0/0/0/3
	// Bundle Interface/Port Channel Bundle-Ether<BundleID>
	// Vlans over Bundle Bundle-Ether<BundleID>.<vlan-id>
	if _, err := ExtractOwnerFromInterfaceName(name); err != nil {
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

			// TODO(sven-rosenzweig): support IPv6 addresses, IPv6 neighbor config

			ipv4, err := NewIPv4(req.Interface.Spec.IPv4)
			if err != nil {
				return err
			}
			iface.IPv4Network = ipv4

			mtu, err := NewMTU(name, req.Interface.Spec.MTU)
			if err != nil {
				return err
			}
			iface.MTUs = mtu

			if req.VRF != nil && req.VRF.Spec.Name != "" {
				iface.Vrf = req.VRF.Spec.Name
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
		iface.Active = "act"
		conf = append(conf, iface)

		return updateInterface(ctx, p.client, conf...)
	case v1alpha1.InterfaceTypeAggregate:
		if err := CheckInterfaceNameTypeAggregate(name); err != nil {
			return err
		}

		iface := Iface{
			Name:           name,
			Description:    req.Interface.Spec.Description,
			Active:         "act",
			ModeNoPhysical: "default",
			Mode:           gnmiext.Empty(false),
		}

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
		}
		return updateInterface(ctx, p.client, conf...)
	case v1alpha1.InterfaceTypeSubinterface:

		// Set Interface mode to virtual is required for bundle interfaces
		iface := Iface{
			Name:           name,
			Description:    req.Interface.Spec.Description,
			Active:         "act",
			Mode:           gnmiext.Empty(false),
			ModeNoPhysical: "default",
		}

		_, subinterfaceID, err := ExtractBundleAndSubinterfaceID(name)
		if err != nil {
			return err
		}

		if subinterfaceID == 0 {
			return fmt.Errorf("no subinterface ID in interfacename specified. pattern: <interface-name>.<subinterface-id>, got %q", name)
		}

		iface.SubInterface = VlanSubInterface{
			VlanIdentifier: VlanIdentifier{
				FirstTag: req.Interface.Spec.Encapsulation.Tag,
				VlanType: "vlan-type-dot1q",
			},
		}

		// Subinterface configures QAndQ vlan
		if req.Interface.Spec.Encapsulation.InnerTag != 0 {
			iface.SubInterface.VlanIdentifier.FirstTag = req.Interface.Spec.Encapsulation.OuterTag
			iface.SubInterface.VlanIdentifier.SecondTag = req.Interface.Spec.Encapsulation.InnerTag
			iface.SubInterface.VlanIdentifier.VlanType = "vlan-type-dot1ad"
		}

		ipv4, err := NewIPv4(req.Interface.Spec.IPv4)
		if err != nil {
			return err
		}
		iface.IPv4Network = ipv4

		if req.VRF.Spec.Name != "" {
			iface.Vrf = req.VRF.Spec.Name
		}

		conf = append(conf, &iface)
		return updateInterface(ctx, p.client, conf...)
	case v1alpha1.InterfaceTypeLoopback, v1alpha1.InterfaceTypeRoutedVLAN:
		return fmt.Errorf("interface type %q is currently not supported", req.Interface.Spec.Type)
	default:
		return fmt.Errorf("unexpected interface type %q", req.Interface.Spec.Type)
	}
}

func NewMTU(intName string, mtu int32) (MTUs, error) {
	owner, err := ExtractOwnerFromInterfaceName(intName)
	if err != nil {
		message := "failed to extract MTU owner from interface name" + intName
		return MTUs{}, errors.New(message)
	}

	mtuValue := mtu
	if mtu == 0 {
		mtuValue = DefaultLinkMTU
	}
	return MTUs{MTU: []MTU{{
		MTU:   mtuValue,
		Owner: string(owner),
	}}}, nil
}

func NewIPv4(ips *v1alpha1.InterfaceIPv4) (IPv4Network, error) {
	if ips == nil || len(ips.Addresses) == 0 {
		return IPv4Network{}, nil
	}

	if len(ips.Addresses) > 1 {
		return IPv4Network{}, errors.New("multiple IPv4 addresses configured for interface, only one is supported")
	}

	prefix := ips.Addresses[0]
	ip := prefix.Addr().String()
	netmask := net.IP(net.CIDRMask(prefix.Bits(), 32)).String()

	return IPv4Network{
		Addresses: AddressesIPv4{
			Primary: Primary{
				Address: ip,
				Netmask: netmask,
			},
		},
		MTU: uint16(DefaultL3MTU),
	}, nil
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

	err := p.client.Delete(ctx, physif)
	if err != nil {
		return fmt.Errorf("failed to delete interface %s: %w", req.Interface.Spec.Name, err)
	}
	return nil
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	state := new(PhysIfState)
	state.Name = req.Interface.Spec.Name

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

func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	vrf := &VRF{
		Name:        req.VRF.Spec.Name,
		Description: req.VRF.Spec.Description,
	}

	for _, routeTarget := range req.VRF.Spec.RouteTargets {
		// Parse the route target value to extract ASN and index
		// TODO(sven-rosenweig): Add support for two-byte (type 0) and IPv4 (type 1) route targets
		// For now we assume that all route targets are in the format of four-byte ASNs with index: <asn>:<index>
		rt, err := NewFourByteRT(routeTarget.Value)
		if err != nil {
			return fmt.Errorf("failed to parse route target %q for VRF %q: %w", routeTarget.Value, req.VRF.Spec.Name, err)
		}
		for _, af := range routeTarget.AddressFamilies {
			switch af {
			case v1alpha1.IPv4:
				AppendAddressFamily(&vrf.AddrFamily.IPv4.Unicast, rt, routeTarget.Action)
			case v1alpha1.IPv6:
				AppendAddressFamily(&vrf.AddrFamily.IPv6.Unicast, rt, routeTarget.Action)
			default:
				return fmt.Errorf("unsupported address family %q for VRF %q", af, req.VRF.Spec.Name)
			}
		}
	}

	return p.client.Update(ctx, vrf)
}

func (p *Provider) DeleteVRF(ctx context.Context, req *provider.VRFRequest) error {
	vrf := &VRF{
		Name: req.VRF.Spec.Name,
	}

	return p.client.Delete(ctx, vrf)
}

func (p *Provider) EnsureBGP(context.Context, *provider.EnsureBGPRequest) error {
	return nil
}

func (p *Provider) DeleteBGP(context.Context, *provider.DeleteBGPRequest) error {
	return nil
}

func (p *Provider) EnsureBGPPeer(ctx context.Context, req *provider.EnsureBGPPeerRequest) error {
	// Ensure that the BGP instance exists and is configured on the "default" domain
	bgp := new(BGP)
	bgp.InstanceName = BGPDefaultInstance
	if err := p.client.GetConfig(ctx, bgp); err != nil {
		return fmt.Errorf("bgp peer: failed to get bgp instance 'default': %w", err)
	}

	if bgp.AS[0].ASNumber != req.BGP.Spec.ASNumber.StrVal {
		return fmt.Errorf("bgp peer: bgp instance 'default' has a different AS number configured (%s) than the one specified in the request (%s)", bgp.AS[0].ASNumber, req.BGP.Spec.ASNumber.StrVal)
	}

	routerID := bgp.AS[0].ASNumber

	// Create Default Route Policies for the peer
	defaultRpl := NewRoutePolicy(req.VRF.Spec.Name)

	err := p.client.Update(ctx, &defaultRpl)
	if err != nil {
		return fmt.Errorf("bgp peer: failed to create route policies: %w", err)
	}

	// Configure BGP Peer

	rd, err := NewRouteDistinguisher(req.VRF.Spec.RouteDistinguisher)
	if err != nil {
		return fmt.Errorf("bgp peer: failed to create route distinguisher: %w", err)
	}

	peer := BGPPeer{
		Name:     req.BGP.Spec.VrfRef.Name,
		RouterID: routerID,
		RD:       rd,
	}

	if req.BGPPeer.Spec.AddressFamilies.Ipv6Unicast != nil || req.BGPPeer.Spec.AddressFamilies.L2vpnEvpn != nil {
		return errors.New("bgp peer: ipv6 unicast or l2vpnEvpn address family is currently not supported")
	}

	// Configure Router Address Family

	routerAF := ActivatedAddressFamilies{
		AF: []ActivatedAddressFamily{
			{
				AFName: string(AfNameIpv4Unicast),
				Redistribute: Redistribute{
					Static: Static{},
				},
			},
		},
	}
	peer.AF = routerAF

	// Configure BGP Neighbor
	neigh := NeighborList{
		[]Neighbor{
			{
				AF: NeighborAddressFamilies{
					AF: []NeighborAddressFamily{
						{
							AfName: "ipv4-unicast",
							RoutePolicy: PeeringRPL{
								In:  defaultRpl.Name,
								Out: defaultRpl.Name,
							},
							// TODO(sven-rosenzweig): make maximum prefix configuration configurable
							MaximumPrefix: MaximumPrefix{
								PrefixLimit: 100,
								Restart:     15,
								Threshold:   80,
							},
						},
					},
				},
				NeighborAddress: req.BGPPeer.Spec.Address,
				RemoteAS:        req.BGPPeer.Spec.ASNumber.IntVal,
				// TODO(sven-rosenzweig): make this configurable
				SessionConfig: SessionConfig{
					SessionGroup: "EBGP-CUSTOMER-DEFAULTS",
				},
				LocalAS: LocalAS{
					AS: AS{
						ASNumber: req.BGPPeer.Spec.LocalAS.ASNumber.IntVal,
						NoPrepend: PrependAS{
							ReplaceAS{},
						},
					},
				},
			},
		},
	}
	peer.Neighbors = neigh

	return p.client.Update(ctx, &peer)
}

func (p *Provider) DeleteBGPPeer(ctx context.Context, req *provider.DeleteBGPPeerRequest) error {
	// Fetch the default BGP instance id
	bgp := new(BGP)
	bgp.InstanceName = BGPDefaultInstance
	if err := p.client.GetConfig(ctx, bgp); err != nil {
		return fmt.Errorf("bgp peer: failed to get bgp instance 'default': %w", err)
	}

	defaultRpl := NewRoutePolicy(req.VRF.Spec.Name)

	peer := BGPPeer{
		RouterID: bgp.AS[0].ASNumber,
		Name:     req.VRF.Spec.Name,
	}

	return p.client.Delete(ctx, &defaultRpl, &peer)
}

func (p *Provider) GetPeerStatus(ctx context.Context, req *provider.BGPPeerStatusRequest) (provider.BGPPeerStatus, error) {
	operState := new(BGPPeerOperStatus)
	operState.Name = req.VRF.Name

	err := p.client.GetState(ctx, operState)
	if err != nil {
		return provider.BGPPeerStatus{}, fmt.Errorf("failed to get BGP peer status %s: %w", operState.Name, err)
	}
	sessionUpTime := time.Now().Unix() - int64(operState.ConnectionUpTime)

	state := provider.BGPPeerStatus{
		SessionState:        operState.State.ToSessionState(),
		LastEstablishedTime: time.Unix(sessionUpTime, 0),
	}

	return state, nil
}

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
