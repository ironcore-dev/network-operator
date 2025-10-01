// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

	if req.Interface.Spec.Type != v1alpha1.InterfaceTypePhysical {
		message := "unsupported interface type for interface " + req.Interface.Spec.Name
		return errors.New(message)
	}

	name := req.Interface.Spec.Name

	physif := &PhysIf{}

	physif.Name = req.Interface.Spec.Name
	physif.Description = req.Interface.Spec.Description

	physif.Statistics.LoadInterval = 30
	owner, err := ExtractMTUOwnerFromIfaceName(name)
	if err != nil {
		message := "failed to extract MTU owner from interface name" + name
		return errors.New(message)
	}
	physif.MTUs = MTUs{MTU: []MTU{{MTU: req.Interface.Spec.MTU, Owner: string(owner)}}}

	// (fixme): for the moment it is enough to keep this static
	// option1: extend existing interface spec
	// option2: create a custom iosxr config
	physif.Shutdown = gnmiext.Empty(false)
	if req.Interface.Spec.AdminState == v1alpha1.AdminStateDown {
		physif.Shutdown = gnmiext.Empty(true)
	}
	physif.Statistics.LoadInterval = uint8(30)

	if len(req.Interface.Spec.IPv4.Addresses) == 0 {
		message := "no IPv4 address configured for interface " + name
		return errors.New(message)
	}

	if len(req.Interface.Spec.IPv4.Addresses) > 1 {
		message := "multiple IPv4 addresses configured for interface " + name
		return errors.New(message)
	}

	// (fixme): support IPv6 addresses, IPv6 neighbor config
	ip := req.Interface.Spec.IPv4.Addresses[0].Addr().String()
	ipNet := req.Interface.Spec.IPv4.Addresses[0].Bits()

	physif.IPv4Network = IPv4Network{
		Addresses: AddressesIPv4{
			Primary: Primary{
				Address: ip,
				Netmask: strconv.Itoa(ipNet),
			},
		},
	}

	// Check if interface exists otherwise patch will fail
	tmpPhysif := &PhysIf{}
	tmpPhysif.Name = name

	err = p.client.GetConfig(ctx, tmpPhysif)
	if err != nil {
		// Interface does not exist, create it
		err = p.client.Update(ctx, physif)
		if err != nil {
			return fmt.Errorf("failed to create interface %s: %w", req.Interface.Spec.Name, err)
		}
		return nil
	}

	err = p.client.Update(ctx, physif)
	if err != nil {
		return err
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

	return provider.InterfaceStatus{
		OperStatus: state.State == string(StateUp),
	}, nil
}

func init() {
	provider.Register("cisco-iosxr-gnmi", NewProvider)
}
