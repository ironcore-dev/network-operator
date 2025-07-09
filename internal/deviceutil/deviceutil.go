// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

// ErrNoDevice is returned when the device label could not be found on the object passed in.
var ErrNoDevice = fmt.Errorf("no %q label present", v1alpha1.DeviceLabel)

func GetDeviceFromMetadata(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*v1alpha1.Device, error) {
	name, ok := obj.Labels[v1alpha1.DeviceLabel]
	if !ok || name == "" {
		return nil, ErrNoDevice
	}
	return GetDeviceByName(ctx, c, obj.Namespace, name)
}

// GetOwnerDevice returns the Device object owning the current resource.
func GetOwnerDevice(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*v1alpha1.Device, error) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind != v1alpha1.DeviceKind {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if gv.Group == v1alpha1.GroupVersion.Group {
			return GetDeviceByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetDeviceByName finds and returns a Device object using the specified selector.
func GetDeviceByName(ctx context.Context, c client.Client, namespace, name string) (*v1alpha1.Device, error) {
	obj := new(v1alpha1.Device)
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s", v1alpha1.GroupVersion.WithKind(v1alpha1.DeviceKind).String(), name)
	}
	return obj, nil
}
