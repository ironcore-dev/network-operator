// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/controller-utils/clientutils"
	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/api"
	"github.com/ironcore-dev/network-operator/internal/provider/edgecore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DeviceFinalizer = "networking.cloud.sap/device"
)

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder           record.EventRecorder
	DiscoverInterfaces bool
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;update;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	device := &v1alpha1.Device{}
	if err := r.Get(ctx, req.NamespacedName, device); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.reconcileExists(ctx, log, device)
}

func (r *DeviceReconciler) reconcileExists(ctx context.Context, log logr.Logger, device *v1alpha1.Device) error {
	if !device.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, device)
	}
	return r.reconcile(ctx, log, device)
}

func (r *DeviceReconciler) delete(ctx context.Context, log logr.Logger, device *v1alpha1.Device) error {
	if _, err := clientutils.PatchEnsureNoFinalizer(ctx, r.Client, device, DeviceFinalizer); err != nil {
		return fmt.Errorf("failed to remove finalizer from device %q: %w", device.Name, err)
	}
	// TODO: Implement deletion logic for the device.
	return nil
}

func (r *DeviceReconciler) reconcile(ctx context.Context, log logr.Logger, device *v1alpha1.Device) error {
	log.Info("Reconciling Device")
	if device.Status.Phase == "" {
		deviceBase := device.DeepCopy()
		deviceBase.Status.Phase = v1alpha1.DevicePhasePending
		if err := r.Status().Patch(ctx, device, client.MergeFrom(device)); err != nil {
			return fmt.Errorf("failed to patch device status: %w", err)
		}
		log.Info("Device status patched", "phase", device.Status.Phase)
		return nil
	}

	if modified, err := clientutils.PatchEnsureFinalizer(ctx, r.Client, device, DeviceFinalizer); err != nil || modified {
		return err
	}
	log.Info("Ensured Finalizer")

	deviceProvider, err := r.getProviderForDevice(device, log)
	if err != nil {
		return fmt.Errorf("failed to get provider for device %q: %w", device.Name, err)
	}
	log.Info("Got Device Provider", "Provider", device.Spec.Provider)

	if err := deviceProvider.Connect(ctx, api.ConnectionDetails{
		Address:  device.Spec.Endpoint.Address,
		Username: "foo",
		Password: "bar",
		Port:     8000,
	}); err != nil {
		return fmt.Errorf("failed to connect to device %q: %w", device.Name, err)
	}
	defer func() {
		if err := deviceProvider.Disconnect(ctx); err != nil {
			log.Error(err, "failed to disconnect from device", "Device", device.Name)
		}
	}()
	log.Info("Connected to Device")

	if r.ensureDeviceStatus(ctx, log, device, deviceProvider) != nil {
		return fmt.Errorf("failed to ensure device status for device %q: %w", device.Name, err)
	}
	log.Info("Ensured Device Status")

	if r.DiscoverInterfaces {
		if err := r.discoverAndCreateInterfacesForDevice(ctx, log, device, deviceProvider); err != nil {
			return fmt.Errorf("failed to discover and create interfaces for device %q: %w", device.Name, err)
		}
		log.Info("Discovered and Created Interfaces for Device")
	}

	if err := r.ensureDeviceSettings(ctx, log, device, deviceProvider); err != nil {
		return fmt.Errorf("failed to ensure device settings for %q: %w", device.Name, err)
	}
	log.Info("Ensured Device Settings")

	// TODO: rest of the story

	log.Info("Reconciled Device")
	return nil
}

func (r *DeviceReconciler) ensureDeviceSettings(ctx context.Context, log logr.Logger, device *v1alpha1.Device, deviceProvider provider.DeviceProvider) error {
	ntpServers := make([]string, 0, len(device.Spec.NTP.Servers))
	for _, server := range device.Spec.NTP.Servers {
		ntpServers = append(ntpServers, server.Address)
	}

	if err := deviceProvider.EnsureDeviceSettings(ctx, api.DeviceSettingsConfig{
		Hostname:       device.Spec.Hostname,
		NTPServers:     ntpServers,
		ProviderConfig: nil,
	}); err != nil {
		return err
	}

	return nil
}

func (r *DeviceReconciler) getProviderForDevice(device *v1alpha1.Device, log logr.Logger) (provider.DeviceProvider, error) {
	var err error
	var deviceProvider provider.DeviceProvider
	switch device.Spec.Provider {
	case v1alpha1.ProviderEdgeCore:
		deviceProvider, err = edgecore.NewProvider(log, edgecore.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create EdgeCore provider: %w", err)
		}
	case v1alpha1.ProviderCisco:
		log.Info("Creating Cisco Provider")
	default:
		return nil, fmt.Errorf("unsupported provider %s", device.Spec.Provider)
	}
	return deviceProvider, nil
}

func (r *DeviceReconciler) ensureDeviceStatus(ctx context.Context, log logr.Logger, device *v1alpha1.Device, deviceProvider provider.DeviceProvider) error {
	deviceInfo, err := deviceProvider.GetDeviceInfo(ctx)
	if err != nil {
		return err
	}

	deviceBase := device.DeepCopy()
	device.Status.Vendor = deviceInfo.Vendor
	device.Status.Model = deviceInfo.Model
	device.Status.SerialNumber = deviceInfo.SerialNumber
	device.Status.OSVersion = deviceInfo.OSVersion

	if err := r.Status().Patch(ctx, device, client.MergeFrom(deviceBase)); err != nil {
		return fmt.Errorf("failed to patch device status: %w", err)
	}

	return nil
}

func (r *DeviceReconciler) discoverAndCreateInterfacesForDevice(ctx context.Context, log logr.Logger, device *v1alpha1.Device, deviceProvider provider.DeviceProvider) error {
	interfaces, err := deviceProvider.ListPhysicalInterfaces(ctx)
	if err != nil {
		return err
	}
	for _, iface := range interfaces {
		log.Info("Creating Interface", "Interface", iface.Name)

		ifaceObj := &v1alpha1.Interface{
			ObjectMeta: metav1.ObjectMeta{
				Name:      iface.Name,
				Namespace: device.Namespace,
			},
			Spec: v1alpha1.InterfaceSpec{
				Name:          iface.Name,
				AdminState:    v1alpha1.AdminState(iface.AdminState),
				Description:   iface.Description,
				Type:          "",
				MTU:           0,
				Switchport:    nil,
				IPv4Addresses: nil,
			},
		}

		if err := controllerutil.SetControllerReference(device, ifaceObj, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference for interface %q: %w", iface.Name, err)
		}

		if err := r.Patch(ctx, ifaceObj, client.Apply, client.FieldOwner("device-controller")); err != nil {
			return fmt.Errorf("failed to apply interface %q: %w", iface.Name, err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Device{}).
		Named("device").
		Owns(&v1alpha1.Interface{}).
		Complete(r)
}
