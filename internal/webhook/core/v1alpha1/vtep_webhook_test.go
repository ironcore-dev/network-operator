// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("VTEP Webhook", func() {
	var (
		obj       *corev1alpha1.VTEP
		oldObj    *corev1alpha1.VTEP
		validator VTEPCustomValidator
	)

	BeforeEach(func() {
		obj = &corev1alpha1.VTEP{
			Spec: corev1alpha1.VTEPSpec{
				DeviceRef:           corev1alpha1.LocalObjectReference{Name: "leaf1"},
				Enabled:             true,
				PrimaryInterfaceRef: corev1alpha1.LocalObjectReference{Name: "lo0"},
				AnycastInterfaceRef: corev1alpha1.LocalObjectReference{Name: "lo1"},
				SuppressARP:         true,
				HostReachability:    corev1alpha1.HostReachabilityTypeFloodAndLearn,
			},
		}
		oldObj = &corev1alpha1.VTEP{}
		validator = VTEPCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("ValidateCreate MulticastGroup", func() {
		It("accepts nil multicastGroup", func() {
			obj.Spec.MulticastGroup = nil
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts valid IPv4 multicast prefix (first & last multicast)", func() {
			obj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("239.1.1.0/24"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects IPv4 prefix with non-multicast first address", func() {
			obj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL3,
				Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/24"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("first address"))
		})

		It("rejects IPv4 prefix whose last address is not multicast (224.0.0.0/3 spans beyond 239.x)", func() {
			obj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL3,
				Prefix: corev1alpha1.MustParsePrefix("224.0.0.0/3"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("last address"))
		})
	})

	Context("Validate Update MulticastGroup IPv4 prefix", func() {
		It("allows unchanged valid multicastGroup", func() {
			oldObj := obj.DeepCopy()
			oldObj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("239.10.10.0/25"),
			}
			newObj := oldObj.DeepCopy()
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects update introducing invalid IPv4 prefix (overlapping with out-of-range on the left)", func() {
			oldObj := obj.DeepCopy()
			oldObj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("239.1.1.0/24"),
			}
			newObj := oldObj.DeepCopy()
			newObj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/24"),
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects update introducing invalid IPv4 prefix (overlapping with out-of-range on the right)", func() {
			oldObj := obj.DeepCopy()
			oldObj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("239.1.1.0/24"),
			}
			newObj := oldObj.DeepCopy()
			newObj.Spec.MulticastGroup = &corev1alpha1.MulticastGroup{
				Type:   corev1alpha1.MulticastGroupTypeL2,
				Prefix: corev1alpha1.MustParsePrefix("224.0.0.0/3"),
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("last address"))
		})
	})

	Context("ValidateDelete", func() {
		It("allows delete on VTEP object", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects delete when object type is wrong", func() {
			_, err := validator.ValidateDelete(ctx, &corev1alpha1.VTEPList{})
			Expect(err).To(HaveOccurred())
		})
	})
})
