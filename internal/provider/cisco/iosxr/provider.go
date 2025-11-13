package iosxr

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
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

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	if p.client == nil {
		return fmt.Errorf("client is not connected")
	}
	var name string = req.Interface.Spec.Name

	var physif *PhisIf = NewIface(name)

	physif.Name = req.Interface.Spec.Name
	physif.Description = req.Interface.Spec.Description

	physif.Statistics.LoadInterval = 30
	owner, err := ExractMTUOwnerFromIfaceName(name)
	if err != nil {
		return fmt.Errorf("failed to extract MTU owner from interface name %s: %w", name, err)
	}
	physif.MTUs = MTUs{MTU: []MTU{{MTU: uint16(req.Interface.Spec.MTU), Owner: string(owner)}}}

	// (fixme): for the moment it is enought to keep this static
	// option1: extend existing interface spec
	// option2: create a custom iosxr config
	physif.Shutdown = Empty(false)
	physif.Statistics.LoadInterval = uint8(30)

	if len(req.Interface.Spec.IPv4.Addresses) == 0 {
		return fmt.Errorf("no IPv4 address configured for interface %s", name)
	}

	if len(req.Interface.Spec.IPv4.Addresses) > 1 {
		return fmt.Errorf("only a single primary IPv4 address is supported for interface %s", name)
	}

	// (fixme): support IPv6 addresses, IPv6 neighbor config
	ip := req.Interface.Spec.IPv4.Addresses[0].Prefix.Addr().String()
	ipNet := req.Interface.Spec.IPv4.Addresses[0].Prefix.Bits()
	if err != nil {
		return fmt.Errorf("failed to parse IPv4 address %s: %w", req.Interface.Spec.IPv4.Addresses[0], err)
	}

	physif.IPv4Network = IPv4Network{
		Addresses: AddressesIPv4{
			Primary: Primary{
				Address: ip,
				Netmask: string(ipNet),
			},
		},
	}

	// Check if interface exists otherwise patch will fail
	var tmpiFace *PhisIf = NewIface(name)
	err = p.client.GetConfig(ctx, tmpiFace)
	if err != nil {
		// Interface does not exist, create it
		err = p.client.Update(ctx, physif)
		if err != nil {
			return fmt.Errorf("failed to create interface %s: %w", req.Interface.Spec.Name, err)
		}
		fmt.Printf("Interface %s created successfully\n", req.Interface.Spec.Name)
		return nil
	}

	err = p.client.Patch(ctx, physif)
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	var iFace = NewIface(req.Interface.Spec.Name)

	if p.client == nil {
		return fmt.Errorf("client is not connected")
	}

	err := p.client.Delete(ctx, iFace)
	if err != nil {
		return fmt.Errorf("failed to delete interface %s: %w", req.Interface.Spec.Name, err)
	}
	return nil
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	state := new(PhysIfState)
	state.Name = req.Interface.Spec.Name

	if p.client == nil {
		return provider.InterfaceStatus{}, fmt.Errorf("client is not connected")
	}

	states, err := p.client.GetStateWithMultipleUpdates(ctx, state)

	if err != nil {
		return provider.InterfaceStatus{}, fmt.Errorf("failed to get interface status for %s: %w", req.Interface.Spec.Name, err)
	}

	providerStatus := provider.InterfaceStatus{
		OperStatus: true,
	}
	for _, s := range *states {
		currState := s.(*PhysIfState)
		if stateMapping[currState.State] != StateUp {
			providerStatus.OperStatus = false
			break
		}
	}
	return providerStatus, nil
}

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
