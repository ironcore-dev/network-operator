// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("IPPrefixPool Webhook", func() {
	var (
		obj       *poolv1alpha1.IPPrefixPool
		oldObj    *poolv1alpha1.IPPrefixPool
		validator IPPrefixPoolCustomValidator
	)

	BeforeEach(func() {
		obj = &poolv1alpha1.IPPrefixPool{
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/16")},
				AllocationPrefixLength: 24,
			},
		}
		oldObj = &poolv1alpha1.IPPrefixPool{}
		validator = IPPrefixPoolCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("ValidateCreate", func() {
		It("accepts valid spec with non-overlapping prefixes", func() {
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("10.1.0.0/16"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts a single prefix", func() {
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts IPv6 prefixes with valid prefixLength", func() {
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("2001:db8::/32"),
			}
			obj.Spec.AllocationPrefixLength = 48
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects overlapping prefixes", func() {
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("10.0.1.0/24"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps"))
		})

		It("rejects prefixLength smaller than or equal to base prefix length", func() {
			obj.Spec.AllocationPrefixLength = 16 // same as /16 base
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be greater than"))
		})

		It("rejects prefixLength smaller than base prefix length", func() {
			obj.Spec.AllocationPrefixLength = 8 // less than /16 base
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be greater than"))
		})

		It("rejects mixed IPv4 and IPv6 prefixes", func() {
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("2001:db8::/32"),
			}
			obj.Spec.AllocationPrefixLength = 48
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("mix IPv4 and IPv6"))
		})

		It("rejects when prefixLength is valid for one prefix but not another", func() {
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("10.1.0.0/28"),
			}
			obj.Spec.AllocationPrefixLength = 24 // valid for /16, invalid for /28
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be greater than"))
		})
	})

	Context("ValidateUpdate", func() {
		It("accepts valid update", func() {
			oldObj = obj.DeepCopy()
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("10.2.0.0/16"),
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects update introducing overlapping prefixes", func() {
			oldObj = obj.DeepCopy()
			obj.Spec.Prefixes = []corev1alpha1.IPPrefix{
				corev1alpha1.MustParsePrefix("10.0.0.0/16"),
				corev1alpha1.MustParsePrefix("10.0.1.0/24"),
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ValidateDelete", func() {
		It("allows deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
