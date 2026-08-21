// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

// ErrNoDevice is returned when the device label could not be found on the object passed in.
var ErrNoDevice = fmt.Errorf("no %q label present", v1alpha1.DeviceLabel)

// ErrNoOwnerDevice is returned when no Device owner reference is found on the object.
var ErrNoOwnerDevice = errors.New("no Device owner reference found")

// GetDeviceFromMetadata resolves the Device for obj by first checking owner
// references, then falling back to the [v1alpha1.DeviceLabel] label.
// Returns an error if the device could not be determined.
func GetDeviceFromMetadata(ctx context.Context, r client.Reader, obj metav1.Object) (*v1alpha1.Device, error) {
	if d, err := GetOwnerDevice(ctx, r, obj); !errors.Is(err, ErrNoOwnerDevice) {
		return d, err
	}
	name, ok := obj.GetLabels()[v1alpha1.DeviceLabel]
	if !ok || name == "" {
		return nil, ErrNoDevice
	}
	return GetDeviceByName(ctx, r, obj.GetNamespace(), name)
}

// GetOwnerDevice returns the Device object owning the current resource.
// Returns [ErrNoOwnerDevice] when no matching owner reference is found.
func GetOwnerDevice(ctx context.Context, r client.Reader, obj metav1.Object) (*v1alpha1.Device, error) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind != v1alpha1.DeviceKind {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if gv.Group == v1alpha1.GroupVersion.Group {
			return GetDeviceByName(ctx, r, obj.GetNamespace(), ref.Name)
		}
	}
	return nil, ErrNoOwnerDevice
}

// GetDeviceByName finds and returns a Device object using the specified selector.
func GetDeviceByName(ctx context.Context, r client.Reader, namespace, name string) (*v1alpha1.Device, error) {
	obj := new(v1alpha1.Device)
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s: %w", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), name, err)
	}
	return obj, nil
}

// DeviceEndpointIPField is the cache field index key used to look up Device objects by
// the IP part of spec.endpoint.address. Callers must register this index on the
// controller-runtime field indexer before calling [GetDeviceByEndpointIP].
const DeviceEndpointIPField = "device.endpoint.ip"

// GetDeviceByEndpointIP finds and returns a Device object by matching the IP portion of
// its spec.endpoint.address against ip. It returns an error if no device is found.
// The caller must have registered the [DeviceEndpointIPField] index before using this function.
func GetDeviceByEndpointIP(ctx context.Context, r client.Reader, ip string) (*v1alpha1.Device, error) {
	deviceList := &v1alpha1.DeviceList{}
	if err := r.List(ctx, deviceList, client.MatchingFields{DeviceEndpointIPField: ip}); err != nil {
		return nil, fmt.Errorf("failed to list %s objects by endpoint IP: %w", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), err)
	}
	if len(deviceList.Items) == 0 {
		return nil, fmt.Errorf("no %s object found with endpoint IP %q", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), ip)
	}
	return &deviceList.Items[0], nil
}

// GetDeviceBySerial finds and returns a Device object using the specified serial number.
// It returns an error if no device or multiple devices with the same serial number are found.
// Note: This function assumes that the [v1alpha1.DeviceSerialLabel] is unique across all Device objects in the cluster.
func GetDeviceBySerial(ctx context.Context, r client.Reader, serial string) (*v1alpha1.Device, error) {
	deviceList := &v1alpha1.DeviceList{}
	listOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{v1alpha1.DeviceSerialLabel: serial}),
	}
	if err := r.List(ctx, deviceList, listOpts); err != nil {
		return nil, fmt.Errorf("failed to list %s objects: %w", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), err)
	}
	if len(deviceList.Items) == 0 {
		return nil, fmt.Errorf("no %s object found with serial %q", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), serial)
	}
	if len(deviceList.Items) > 1 {
		return nil, fmt.Errorf("multiple %s objects found with serial %q", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), serial)
	}
	return &deviceList.Items[0], nil
}

// Connection holds the necessary information to connect to a device's API.
type Connection struct {
	// Address is the API address of the device, in the format "host:port".
	Address string
	// Username for basic authentication. Might be empty if the device does not require authentication.
	Username string
	// Password for basic authentication. Might be empty if the device does not require authentication.
	Password string `json:"-"`
	// TLS configuration for the connection.
	TLS *tls.Config
}

// GetDeviceConnection retrieves the connection details for accessing the Device.
func GetDeviceConnection(ctx context.Context, r client.Reader, obj *v1alpha1.Device) (*Connection, error) {
	c := clientutil.NewClient(r, obj.Namespace)

	conf := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	if obj.Spec.Endpoint.TLS != nil {
		ca, err := c.Secret(ctx, &obj.Spec.Endpoint.TLS.CA)
		if err != nil {
			return nil, err
		}

		log := ctrl.LoggerFrom(ctx)
		if certPool := x509.NewCertPool(); certPool.AppendCertsFromPEM(ca) {
			log.V(2).Info("added CA certificate to x509 pool")
			conf = &tls.Config{RootCAs: certPool, MinVersion: tls.VersionTLS12}

			if obj.Spec.Endpoint.TLS.Certificate != nil {
				cert, err := c.Certificate(ctx, &obj.Spec.Endpoint.TLS.Certificate.SecretRef)
				if err != nil {
					return nil, err
				}
				log.V(2).Info("added client certificate tls configuration")
				conf.Certificates = []tls.Certificate{*cert}
			}
		} else {
			log.Error(errors.New("failed to append CA certificate to x509 pool"), "")
		}
	}

	var user, pass []byte
	if obj.Spec.Endpoint.SecretRef != nil {
		var err error
		user, pass, err = c.BasicAuth(ctx, obj.Spec.Endpoint.SecretRef)
		if err != nil {
			return nil, err
		}
	}

	return &Connection{
		Address:  obj.Spec.Endpoint.Address,
		Username: string(user),
		Password: string(pass),
		TLS:      conf,
	}, nil
}
