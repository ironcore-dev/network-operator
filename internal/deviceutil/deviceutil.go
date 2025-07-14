// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

// ErrNoDevice is returned when the device label could not be found on the object passed in.
var ErrNoDevice = fmt.Errorf("no %q label present", v1alpha1.DeviceLabel)

func GetDeviceFromMetadata(ctx context.Context, r client.Reader, obj metav1.Object) (*v1alpha1.Device, error) {
	if d, err := GetOwnerDevice(ctx, r, obj); err == nil && d != nil {
		return d, nil
	}
	name, ok := obj.GetLabels()[v1alpha1.DeviceLabel]
	if !ok || name == "" {
		return nil, ErrNoDevice
	}
	return GetDeviceByName(ctx, r, obj.GetNamespace(), name)
}

// GetOwnerDevice returns the Device object owning the current resource.
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
	return nil, nil
}

// GetDeviceByName finds and returns a Device object using the specified selector.
func GetDeviceByName(ctx context.Context, r client.Reader, namespace, name string) (*v1alpha1.Device, error) {
	obj := new(v1alpha1.Device)
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), name)
	}
	return obj, nil
}

func GetDeviceGrpcClient(ctx context.Context, r client.Reader, obj *v1alpha1.Device) (*grpc.ClientConn, error) {
	c := clientutil.NewClient(r, obj.Namespace)

	conf := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	if obj.Spec.Endpoint.TLS != nil {
		ca, err := c.Secret(ctx, obj.Spec.Endpoint.TLS.CA)
		if err != nil {
			return nil, err
		}

		log := ctrl.LoggerFrom(ctx)
		if certPool := x509.NewCertPool(); certPool.AppendCertsFromPEM(ca) {
			log.Info("added CA certificate to x509 pool")
			conf = &tls.Config{RootCAs: certPool, MinVersion: tls.VersionTLS12}

			if obj.Spec.Endpoint.TLS.Certificate != nil {
				cert, err := c.Certificate(ctx, obj.Spec.Endpoint.TLS.Certificate.SecretRef)
				if err != nil {
					return nil, err
				}
				log.Info("added client certificate tls configuration")
				conf.Certificates = []tls.Certificate{*cert}
			}
		} else {
			log.Error(errors.New("failed to append CA certificate to x509 pool"), "")
		}
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(conf))}
	if obj.Spec.Endpoint.SecretRef != nil {
		user, pass, err := c.BasicAuth(ctx, obj.Spec.Endpoint.SecretRef)
		if err != nil {
			return nil, err
		}

		opts = append(opts, grpc.WithPerRPCCredentials(&auth{
			Username: string(user),
			Password: string(pass),
		}))
	}

	return grpc.NewClient(obj.Spec.Endpoint.Address, opts...)
}

type auth struct {
	Username string
	Password string
}

var _ credentials.PerRPCCredentials = (*auth)(nil)

func (a *auth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"username": a.Username,
		"password": a.Password,
	}, nil
}

func (a *auth) RequireTransportSecurity() bool { return true }
