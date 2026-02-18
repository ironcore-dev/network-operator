// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

func GetDeviceBySerial(ctx context.Context, r client.Reader, namespace, serial string) (*v1alpha1.Device, error) {
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
//
// TODO(felix-kaestner): find a better place for this struct, maybe in a 'connection' package?
type Connection struct {
	// Address is the API address of the device, in the format "host:port".
	Address string
	// Username for basic authentication. Might be empty if the device does not require authentication.
	Username string
	// Password for basic authentication. Might be empty if the device does not require authentication.
	Password string // #nosec G117
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
			log.Info("added CA certificate to x509 pool")
			conf = &tls.Config{RootCAs: certPool, MinVersion: tls.VersionTLS12}

			if obj.Spec.Endpoint.TLS.Certificate != nil {
				cert, err := c.Certificate(ctx, &obj.Spec.Endpoint.TLS.Certificate.SecretRef)
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

// NewGrpcClient creates a new gRPC client connection to a specified device using the provided [Connection].
// The connection will use TLS if the [Connection.TLS] field is set, otherwise it will use an insecure connection.
// If the [Connection.Username] and [Connection.Password] fields are set, basic authentication in the form of metadata will be used.
func NewGrpcClient(ctx context.Context, conn *Connection, o ...Option) (*grpc.ClientConn, error) {
	creds := insecure.NewCredentials()
	if conn.TLS != nil {
		creds = credentials.NewTLS(conn.TLS)
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if conn.Username != "" && conn.Password != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&auth{
			Username: conn.Username,
			Password: conn.Password,
		}))
	}

	for _, opt := range o {
		dialOpt, err := opt()
		if err != nil {
			return nil, err
		}
		opts = append(opts, dialOpt)
	}

	return grpc.NewClient(conn.Address, opts...)
}

type Option func() (grpc.DialOption, error)

// WithDefaultTimeout returns a gRPC dial option that sets a default timeout for each RPC.
// If a deadline is already present in the context, it will not be modified.
func WithDefaultTimeout(timeout time.Duration) Option {
	return func() (grpc.DialOption, error) {
		if timeout <= 0 {
			return nil, errors.New("timeout must be greater than zero")
		}
		return grpc.WithUnaryInterceptor(UnaryDefaultTimeoutInterceptor(timeout)), nil
	}
}

type auth struct {
	Username             string
	Password             string // #nosec G117
	SecureTransportCreds bool
}

var _ credentials.PerRPCCredentials = (*auth)(nil)

func (a *auth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"username": a.Username,
		"password": a.Password,
	}, nil
}

func (a *auth) RequireTransportSecurity() bool {
	// Only called if the transport credentials are insecure.
	return false
}

// UnaryDefaultTimeoutInterceptor returns a gRPC unary client interceptor that sets a default timeout
// for each RPC. If a deadline is already present , it will not be modified.
func UnaryDefaultTimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); ok {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
