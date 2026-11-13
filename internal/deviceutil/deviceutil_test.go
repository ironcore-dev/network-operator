// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
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
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
)

func TestGetDeviceFromMetadata(t *testing.T) {
	g := NewWithT(t)

	device := &v1alpha1.Device{
		Name:      "test-device",
		Namespace: metav1.NamespaceDefault,
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
		Name:      "test-device",
		Namespace: metav1.NamespaceDefault,
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
		Name:      "test-device",
		Namespace: metav1.NamespaceDefault,
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
		Name:      "test-device",
		Namespace: metav1.NamespaceDefault,
		Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "SER-001"},
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
		Name:      "test-device",
		Namespace: metav1.NamespaceDefault,
		Spec:      v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "10.0.0.10:22"}},
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

func TestFabricSelection(t *testing.T) {
	newFixtures := func() (map[string]*v1alpha1.Device, []evpnv1alpha1.Fabric) {
		devices := map[string]*v1alpha1.Device{
			"leaf1": {Name: "leaf1", Labels: map[string]string{"fabric": "fabric1"}},
			"leaf2": {Name: "leaf2", Labels: map[string]string{"fabric": "fabric1"}},
		}
		fabrics := []evpnv1alpha1.Fabric{
			{Name: "fabric1", Spec: evpnv1alpha1.FabricSpec{DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"fabric": "fabric1"}}}},
			{Name: "fabric2", Spec: evpnv1alpha1.FabricSpec{DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"fabric": "fabric2"}}}},
		}
		return devices, fabrics
	}

	t.Run("selects the common Fabric without requiring iBGP configuration", func(t *testing.T) {
		g := NewWithT(t)
		devices, fabrics := newFixtures()
		fabric, err := SelectFabric(devices, fabrics)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(fabric).To(BeIdenticalTo(&fabrics[0]))
	})

	t.Run("rejects empty device sets", func(t *testing.T) {
		g := NewWithT(t)
		_, fabrics := newFixtures()
		fabric, err := SelectFabric(nil, fabrics)
		g.Expect(err).To(MatchError(ContainSubstring("no devices")))
		g.Expect(fabric).To(BeNil())
	})

	t.Run("reports unmatched Devices", func(t *testing.T) {
		g := NewWithT(t)
		devices, _ := newFixtures()
		fabric, err := SelectFabric(devices, nil)
		g.Expect(err).To(MatchError(ContainSubstring(`device "leaf1" does not match any fabric`)))
		g.Expect(err).To(MatchError(ContainSubstring(`device "leaf2" does not match any fabric`)))
		g.Expect(fabric).To(BeNil())
	})

	t.Run("reports overlapping Fabric selectors", func(t *testing.T) {
		g := NewWithT(t)
		devices, fabrics := newFixtures()
		fabrics[1].Spec.DeviceSelector = fabrics[0].Spec.DeviceSelector
		fabric, err := SelectFabric(devices, fabrics)
		g.Expect(err).To(MatchError(ContainSubstring("matches 2 fabrics")))
		g.Expect(err).To(MatchError(ContainSubstring("want exactly one")))
		g.Expect(fabric).To(BeNil())
	})

	t.Run("rejects Devices in different Fabrics", func(t *testing.T) {
		g := NewWithT(t)
		devices, fabrics := newFixtures()
		devices["leaf2"].Labels["fabric"] = "fabric2"
		fabric, err := SelectFabric(devices, fabrics)
		g.Expect(err).To(MatchError(ContainSubstring(`device "leaf2" resolves to fabric "fabric2", want "fabric1"`)))
		g.Expect(fabric).To(BeNil())
	})

	t.Run("reports invalid selectors with the Fabric name", func(t *testing.T) {
		g := NewWithT(t)
		devices, fabrics := newFixtures()
		fabrics[1].Spec.DeviceSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{Key: "fabric", Operator: "invalid"}}
		fabric, err := SelectFabric(devices, fabrics)
		g.Expect(err).To(MatchError(ContainSubstring(`parse device selector for fabric: "fabric2"`)))
		g.Expect(fabric).To(BeNil())
	})
}

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}
