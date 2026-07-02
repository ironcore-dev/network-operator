// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("BGPPeer Webhook", func() {
	var (
		obj       *v1alpha1.BGPPeer
		oldObj    *v1alpha1.BGPPeer
		validator BGPPeerCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.BGPPeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-bgppeer",
				Namespace: "default",
			},
			Spec: v1alpha1.BGPPeerSpec{
				DeviceRef: v1alpha1.LocalObjectReference{
					Name: "test-device",
				},
				Address: "192.168.1.1",
			},
		}
		oldObj = obj.DeepCopy()
		validator = BGPPeerCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("When creating BGPPeer under Validating Webhook", func() {
		It("Should admit creation with valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with valid string AS number in plain format", func() {
			obj.Spec.ASNumber = intstr.FromString("4294967295")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with valid string AS number in dotted notation", func() {
			obj.Spec.ASNumber = intstr.FromString("65000.100")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with minimum valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(1)
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with maximum valid integer AS number", func() {
			obj.Spec.ASNumber = intstr.FromInt32(2147483647)
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

		It("Should admit creation with dotted notation - low part is zero", func() {
			obj.Spec.ASNumber = intstr.FromString("100.0")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with dotted notation at boundary values", func() {
			obj.Spec.ASNumber = intstr.FromString("65535.65535")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with dotted notation at minimum valid values", func() {
			obj.Spec.ASNumber = intstr.FromString("1.0")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating BGPPeer with deprecated LocalASNumber", func() {
		BeforeEach(func() {
			// ASNumber is required for BGPPeer validation
			obj.Spec.ASNumber = intstr.FromInt32(65001)
		})

		It("Should return deprecation warning when LocalASNumber is set", func() {
			localAS := intstr.FromInt32(65001)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0]).To(ContainSubstring("deprecated"))
		})

		It("Should not return warning when LocalASNumber is not set", func() {
			obj.Spec.LocalASNumber = nil //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should admit creation with valid integer LocalASNumber and return warning", func() {
			localAS := intstr.FromInt32(65001)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with valid string LocalASNumber in plain format and return warning", func() {
			localAS := intstr.FromString("4294967295")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with valid string LocalASNumber in dotted notation and return warning", func() {
			localAS := intstr.FromString("65000.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with minimum valid integer LocalASNumber and return warning", func() {
			localAS := intstr.FromInt32(1)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with maximum valid integer LocalASNumber and return warning", func() {
			localAS := intstr.FromInt32(2147483647)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should deny creation with zero LocalASNumber", func() {
			localAS := intstr.FromInt32(0)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with negative LocalASNumber", func() {
			localAS := intstr.FromInt32(-1)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with string LocalASNumber exceeding uint32 max", func() {
			localAS := intstr.FromString("4294967296")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid string LocalASNumber", func() {
			localAS := intstr.FromString("invalid")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation LocalASNumber - too many parts", func() {
			localAS := intstr.FromString("1.2.3")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation LocalASNumber - high part out of range", func() {
			localAS := intstr.FromString("65536.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation LocalASNumber - low part out of range", func() {
			localAS := intstr.FromString("100.65536")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with invalid dotted notation LocalASNumber - high part is zero", func() {
			localAS := intstr.FromString("0.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit creation with dotted notation LocalASNumber - low part is zero and return warning", func() {
			localAS := intstr.FromString("100.0")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with dotted notation LocalASNumber at boundary values and return warning", func() {
			localAS := intstr.FromString("65535.65535")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit creation with dotted notation LocalASNumber at minimum valid values and return warning", func() {
			localAS := intstr.FromString("1.0")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})
	})

	Context("When updating BGPPeer under Validating Webhook", func() {
		It("Should admit update with valid AS number", func() {
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

		It("Should admit update from integer to string format", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromString("4000000000")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit update from plain to dotted notation", func() {
			oldObj.Spec.ASNumber = intstr.FromString("4000000000")
			obj.Spec.ASNumber = intstr.FromString("61035.36928")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When updating BGPPeer with deprecated LocalASNumber", func() {
		It("Should return deprecation warning when LocalASNumber is set", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromInt32(65100)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0]).To(ContainSubstring("deprecated"))
		})

		It("Should admit update with valid integer LocalASNumber and return warning", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromInt32(65100)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit update with valid string LocalASNumber in plain format and return warning", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("4294967295")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit update with valid string LocalASNumber in dotted notation and return warning", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("65000.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should deny update with zero LocalASNumber", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromInt32(0)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with negative LocalASNumber", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromInt32(-1)
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with string LocalASNumber exceeding uint32 max", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("4294967296")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with invalid string LocalASNumber", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("invalid")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with invalid dotted notation LocalASNumber - too many parts", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("1.2.3")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with invalid dotted notation LocalASNumber - high part out of range", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("65536.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with invalid dotted notation LocalASNumber - low part out of range", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("100.65536")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update with invalid dotted notation LocalASNumber - high part is zero", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("0.100")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit update with dotted notation LocalASNumber - low part is zero and return warning", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("100.0")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})

		It("Should admit update with dotted notation LocalASNumber at boundary values and return warning", func() {
			oldObj.Spec.ASNumber = intstr.FromInt32(65001)
			obj.Spec.ASNumber = intstr.FromInt32(65001)
			localAS := intstr.FromString("65535.65535")
			obj.Spec.LocalASNumber = &localAS //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(HaveLen(1))
		})
	})

	Context("When deleting BGPPeer under Validating Webhook", func() {
		It("Should admit deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
