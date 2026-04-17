// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package deviceutil

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
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

	// Owner reference pointing to a non-existent Device should
	// propagate the API error instead of falling through to label lookup.
	obj = metav1.ObjectMeta{
		OwnerReferences: []metav1.OwnerReference{
			{
				Kind:       v1alpha1.DeviceKind,
				APIVersion: v1alpha1.GroupVersion.String(),
				Name:       "non-existent-device",
			},
		},
		Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
		Namespace: metav1.NamespaceDefault,
	}
	d, err = GetDeviceFromMetadata(t.Context(), client, &obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(d).To(BeNil())
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

	// No Device owner reference at all.
	obj.OwnerReferences = nil
	d, err = GetOwnerDevice(t.Context(), client, &obj)
	g.Expect(errors.Is(err, ErrNoOwnerDevice)).To(BeTrue())
	g.Expect(d).To(BeNil())

	// Owner reference with a different Kind.
	obj.OwnerReferences = []metav1.OwnerReference{
		{
			Kind:       "Other",
			APIVersion: v1alpha1.GroupVersion.String(),
			Name:       "test-device",
		},
	}
	d, err = GetOwnerDevice(t.Context(), client, &obj)
	g.Expect(errors.Is(err, ErrNoOwnerDevice)).To(BeTrue())
	g.Expect(d).To(BeNil())
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

func TestGetDeviceBySerial(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: metav1.NamespaceDefault,
			Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "SER-001"},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device).
		Build()

	d, err := GetDeviceBySerial(t.Context(), client, "SER-001")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())

	d, err = GetDeviceBySerial(t.Context(), client, "SER-404")
	g.Expect(err).To(HaveOccurred())
	g.Expect(d).To(BeNil())
}

func TestGetDeviceByEndpointIP(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "10.0.0.10:22"}},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device).
		WithIndex(&v1alpha1.Device{}, DeviceEndpointIPField, func(obj ctrlclient.Object) []string {
			d := obj.(*v1alpha1.Device)
			ip, _, _ := strings.Cut(d.Spec.Endpoint.Address, ":")
			if ip == "" {
				return nil
			}
			return []string{ip}
		}).
		Build()

	d, err := GetDeviceByEndpointIP(t.Context(), client, "10.0.0.10")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(d).NotTo(BeNil())

	d, err = GetDeviceByEndpointIP(t.Context(), client, "10.0.0.99")
	g.Expect(err).To(HaveOccurred())
	g.Expect(d).To(BeNil())
}

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}
