// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("NetworkVirtualizationEdge Webhook", func() {
	var (
		obj       *corev1alpha1.NetworkVirtualizationEdge
		oldObj    *corev1alpha1.NetworkVirtualizationEdge
		validator NetworkVirtualizationEdgeCustomValidator
	)

	BeforeEach(func() {
		obj = &corev1alpha1.NetworkVirtualizationEdge{
			Spec: corev1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:                 corev1alpha1.LocalObjectReference{Name: "leaf1"},
				AdminState:                corev1alpha1.AdminStateUp,
				SourceInterfaceRef:        corev1alpha1.LocalObjectReference{Name: "lo0"},
				AnycastSourceInterfaceRef: &corev1alpha1.LocalObjectReference{Name: "lo1"},
				SuppressARP:               true,
				HostReachability:          corev1alpha1.HostReachabilityTypeFloodAndLearn,
			},
		}
		oldObj = &corev1alpha1.NetworkVirtualizationEdge{}
		validator = NetworkVirtualizationEdgeCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("ValidateCreate MulticastGroup", func() {
		It("accepts nil multicastGroup", func() {
			obj.Spec.MulticastGroups = nil
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts valid IPv4 multicast address", func() {
			obj.Spec.MulticastGroups = &corev1alpha1.MulticastGroups{
				L2: "239.1.1.1",
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects non-multicast IPv4 address", func() {
			obj.Spec.MulticastGroups = &corev1alpha1.MulticastGroups{
				L3: "10.0.0.1",
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Validate Update MulticastGroup IPv4 prefix", func() {
		It("allows unchanged valid multicastGroup", func() {
			oldObj := obj.DeepCopy()
			oldObj.Spec.MulticastGroups = &corev1alpha1.MulticastGroups{
				L2: "239.10.10.1",
			}
			newObj := oldObj.DeepCopy()
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("ValidateDelete", func() {
		It("allows delete on NVE object", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects delete when object type is wrong", func() {
			_, err := validator.ValidateDelete(ctx, &corev1alpha1.NetworkVirtualizationEdgeList{})
			Expect(err).To(HaveOccurred())
		})
	})
})
