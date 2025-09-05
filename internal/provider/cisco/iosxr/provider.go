// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package iosxr

import (
	"context"
	"errors"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/iosxr/gnmi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Provider struct{}

func (p *Provider) CreateDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("dcaas router creation called", "device", device.Name)

	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}

	conn, err := deviceutil.GetDeviceGrpcClient(ctx, c, device)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	defer conn.Close()

	gnmi, err := gnmi.NewClient(ctx, gpb.NewGNMIClient(conn), true)
	if err != nil {
		log.Error(err, "Failed to connect to device")
		return fmt.Errorf("failed to create gnmi client: %w", err)
	}

	log.Info("Try to created gRPC connection to device", "device", device.Name)
	gnmi.Get(ctx, "Cisco-IOS-XR-ifmgr-cfg:interface-configurations/interface-configuration")

	log.Info("Successfully created gRPC connection to device", "device", device.Name)

	return nil
}

func (p *Provider) DeleteDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DeleteDevice is not implemented for iosxr provider")
	return nil
}

func (p *Provider) CreateInterface(ctx context.Context, intf *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("CreateInterface is not implemented for iosxr provider")
	return nil
}

func (p *Provider) DeleteInterface(ctx context.Context, intf *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DeleteInterface is not implemented for iosxr provider")
	return nil
}

func init() {
	log := ctrl.Log.WithName("iosxr")
	log.Info("Registering iosxr provider")
	provider.Register("cisco-ios-xr-gnmi", &Provider{})
}
