// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("BGP Webhook", func() {
	var (
		obj       *v1alpha1.BGP
		oldObj    *v1alpha1.BGP
		validator BGPCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.BGP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-bgp",
				Namespace: "default",
			},
			Spec: v1alpha1.BGPSpec{
				DeviceRef: v1alpha1.LocalObjectReference{
					Name: "test-device",
				},
				RouterID: "10.0.0.1",
			},
		}
		oldObj = obj.DeepCopy()
		validator = BGPCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("When creating BGP under Validating Webhook", func() {
		It("Should allow creation with valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with valid string AS number in plain format", func() {
			obj.Spec.ASNumber = intstr.FromString("4294967295")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with valid string AS number in dotted notation", func() {
			obj.Spec.ASNumber = intstr.FromString("65000.100")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with minimum valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(1)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with maximum valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(math.MaxInt32)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation with zero AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(0)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with negative AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(-1)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with string AS number exceeding uint32 max", func() {
			obj.Spec.ASNumber = intstr.FromString("4294967296")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid string AS number", func() {
			obj.Spec.ASNumber = intstr.FromString("invalid")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation - too many parts", func() {
			obj.Spec.ASNumber = intstr.FromString("1.2.3")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation - high part out of range", func() {
			obj.Spec.ASNumber = intstr.FromString("65536.100")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation - low part out of range", func() {
			obj.Spec.ASNumber = intstr.FromString("100.65536")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation - high part is zero", func() {
			obj.Spec.ASNumber = intstr.FromString("0.100")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should allow creation with dotted notation - low part is zero", func() {
			obj.Spec.ASNumber = intstr.FromString("100.0")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with dotted notation at boundary values", func() {
			obj.Spec.ASNumber = intstr.FromString("65535.65535")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow creation with dotted notation at minimum valid values", func() {
			obj.Spec.ASNumber = intstr.FromString("1.0")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When updating BGP under Validating Webhook", func() {
		It("Should allow update with valid AS number", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65002)
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny update with invalid AS number", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(0)
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should allow update from integer to string format", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromString("4000000000")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow update from plain to dotted notation", func() {
			oldObj.Spec.ASNumber = intstr.FromString("4000000000")
			obj.Spec.ASNumber = intstr.FromString("61035.36928")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting BGP under Validating Webhook", func() {
		It("Should allow deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
