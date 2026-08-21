// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("IndexPool Webhook", func() {
	var (
		obj       *poolv1alpha1.IndexPool
		oldObj    *poolv1alpha1.IndexPool
		validator IndexPoolCustomValidator
	)

	BeforeEach(func() {
		obj = &poolv1alpha1.IndexPool{
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("100..200")},
			},
		}
		oldObj = &poolv1alpha1.IndexPool{}
		validator = IndexPoolCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("ValidateCreate", func() {
		It("accepts non-overlapping ranges", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..200"),
				corev1alpha1.MustParseIndexRange("300..400"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts a single range", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("64512..65534"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts adjacent but non-overlapping ranges", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..199"),
				corev1alpha1.MustParseIndexRange("200..299"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects overlapping ranges", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..200"),
				corev1alpha1.MustParseIndexRange("150..250"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps"))
		})

		It("rejects a range contained within another", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..300"),
				corev1alpha1.MustParseIndexRange("150..200"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps"))
		})

		It("rejects ranges sharing a single boundary value", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..200"),
				corev1alpha1.MustParseIndexRange("200..300"),
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps"))
		})

		It("rejects a range with end equal to math.MaxInt64", func() {
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				{Start: 0, End: 9223372036854775807},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("MaxInt64"))
		})
	})

	Context("ValidateUpdate", func() {
		It("accepts valid update with non-overlapping ranges", func() {
			oldObj = obj.DeepCopy()
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..200"),
				corev1alpha1.MustParseIndexRange("500..600"),
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects update introducing overlapping ranges", func() {
			oldObj = obj.DeepCopy()
			obj.Spec.Ranges = []corev1alpha1.IndexRange{
				corev1alpha1.MustParseIndexRange("100..200"),
				corev1alpha1.MustParseIndexRange("180..250"),
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
