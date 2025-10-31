// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

func TestGetDeviceFromMetadata(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: metav1.NamespaceDefault,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device).
		Build()

	obj := metav1.ObjectMeta{
		Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
		Namespace: metav1.NamespaceDefault,
	}

	d, err := GetDeviceFromMetadata(t.Context(), client, &obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())

	obj = metav1.ObjectMeta{
		OwnerReferences: []metav1.OwnerReference{
			{
				Kind:       v1alpha1.DeviceKind,
				APIVersion: v1alpha1.GroupVersion.String(),
				Name:       "test-device",
			},
		},
		Namespace: metav1.NamespaceDefault,
		Name:      "resource-owned-by-device",
	}

	d, err = GetOwnerDevice(t.Context(), client, &obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())
}

func TestGetOwnerDevice(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: metav1.NamespaceDefault,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device).
		Build()

	obj := metav1.ObjectMeta{
		OwnerReferences: []metav1.OwnerReference{
			{
				Kind:       v1alpha1.DeviceKind,
				APIVersion: v1alpha1.GroupVersion.String(),
				Name:       "test-device",
			},
		},
		Namespace: metav1.NamespaceDefault,
		Name:      "resource-owned-by-device",
	}

	d, err := GetOwnerDevice(t.Context(), client, &obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())

	obj.OwnerReferences[0].APIVersion = "networking.metal.ironcore.dev/v1alpha1234"
	d, err = GetOwnerDevice(t.Context(), client, &obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())
}

func TestGetDeviceByName(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: metav1.NamespaceDefault,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device).
		Build()

	d, err := GetDeviceByName(t.Context(), client, metav1.NamespaceDefault, "test-device")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())

	d, err = GetDeviceByName(t.Context(), client, metav1.NamespaceDefault, "non-existent-device")
	g.Expect(err).To(HaveOccurred())
	g.Expect(d).To(BeNil())
}

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}
