// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package sonic

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var (
	_ provider.Provider          = &Provider{}
	_ provider.InterfaceProvider = &Provider{}
)

type Provider struct {
	initConfig *provider.SonicProviderInitConfig

	conn   *grpc.ClientConn
}

func NewProvider(initConfig provider.ProviderInitConfig) provider.Provider {
	return &Provider{
		initConfig: initConfig.(*provider.SonicProviderInitConfig),
	}
}


func (p *Provider) Init(ctx context.Context) (err error) {
	// Create a fake connection for testing/development
	address := fmt.Sprintf("%s:%d", p.initConfig.Address, p.initConfig.Port)
	
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithInsecure(), // Use for testing only
		grpc.WithBlock(),
		grpc.WithTimeout(time.Second*5),
	)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	
	p.conn = conn

	return nil
}

// func (p *Provider) Disconnect(context.Context, config *provider.ProviderConfig) error {
// 	return p.conn.Close()
// }

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) (provider.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if err := p.Init(ctx); err != nil {
		return provider.Result{}, fmt.Errorf("failed to initialize provider: %w", err)
	}
	defer func() {
		if disconnectErr := p.conn.Close(); disconnectErr != nil {
			log.Error(disconnectErr, "failed to close grpc connection")
		}
	}()

	return provider.Result{}, nil
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	if err := p.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize provider: %w", err)
	}
	defer func() error{
		if disconnectErr := p.conn.Close(); disconnectErr != nil {
			return fmt.Errorf("failed to close grpc connection: %w", disconnectErr)
		}
		return nil
	} ()

	// switch req.Interface.Spec.Type {
	// case v1alpha1.InterfaceTypePhysical:
	// 	// For physical interfaces, we can't delete the interface directly.
	// 	// Instead, we reset the configuration and set the admin state down.
	// 	sb := new(ygnmi.SetBatch)
	// 	ygnmi.BatchUpdate(sb, Root().Interface(req.Interface.Spec.Name).Enabled().Config(), false)
	// 	ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Description().Config())
	// 	ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).SubinterfaceMap().Config())
	// 	ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Ethernet().Config())
	// 	ygnmi.BatchDelete(sb, Root().Interface(req.Interface.Spec.Name).Ethernet().SwitchedVlan().Config())
	// 	_, err := sb.Set(ctx, p.client, ygnmi.WithEncoding(gpb.Encoding_JSON), ygnmi.WithAppendModuleName(true))
	// 	return err
	// case v1alpha1.InterfaceTypeLoopback:
	// 	_, err := ygnmi.Delete(ctx, p.client, Root().Interface(req.Interface.Spec.Name).Config())
	// 	return err
	// }

	return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
}


func init() {
	provider.Register(v1alpha1.SonicProviderType, NewProvider)
}
