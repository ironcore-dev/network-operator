// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("VRF Webhook", func() {
	var (
		obj       *v1alpha1.VRF
		oldObj    *v1alpha1.VRF
		validator VRFCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.VRF{
			Spec: v1alpha1.VRFSpec{
				Name:      "TEST",
				DeviceRef: v1alpha1.LocalObjectReference{Name: "leaf1"},
			},
		}
		oldObj = &v1alpha1.VRF{}
		validator = VRFCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("ValidateCreate RouteDistinguisher", func() {
		It("accepts empty RD", func() {
			obj.Spec.RouteDistinguisher = ""
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects bad format (missing colon)", func() {
			obj.Spec.RouteDistinguisher = "badformat"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("format must be"))
		})

		It("accepts type0 lower bounds", func() {
			obj.Spec.RouteDistinguisher = "1:0"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts type0 upper bounds", func() {
			obj.Spec.RouteDistinguisher = "65534:4294967295"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects reserved ASN (0)", func() {
			obj.Spec.RouteDistinguisher = "0:10"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reserved"))
		})

		It("accepts type1 IPv4 administrator", func() {
			obj.Spec.RouteDistinguisher = "10.0.0.1:65000"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects type1 assigned overflow", func() {
			obj.Spec.RouteDistinguisher = "10.0.0.1:70000"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-1"))
		})

		It("accepts type2 bounds", func() {
			obj.Spec.RouteDistinguisher = "65536:65535"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects type2 assigned overflow", func() {
			obj.Spec.RouteDistinguisher = "70000:70000"
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-2"))
		})
	})

	Context("ValidateUpdate RouteDistinguisher", func() {
		It("allows unchanged RD", func() {
			oldObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "1:1"},
			}
			newObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "1:1"},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows changed RD (no immutability enforced)", func() {
			oldObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "1:1"},
			}
			newObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "65536:2"},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects update to invalid RD", func() {
			oldObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "1:1"},
			}
			newObj := &v1alpha1.VRF{
				Spec: v1alpha1.VRFSpec{RouteDistinguisher: "bad-format"},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ValidateCreate RouteTargets", func() {
		It("accepts valid type-0 route target", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "65000:4294967295"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts valid type-1 route target", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "192.0.2.1:65535"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts valid type-2 route target", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "65536:10"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects IPv4 assigned overflow", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "10.0.0.1:70000"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-1"))
		})

		It("rejects reserved ASN 0", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "0:10"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reserved"))
		})

		It("rejects reserved ASN 65535", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "65535:10"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reserved"))
		})

		It("rejects reserved ASN 4294967295", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "4294967295:10"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reserved"))
		})

		It("rejects type-2 assigned overflow", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "70000:70000"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-2"))
		})

		It("rejects bad format (missing colon)", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "badformat"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("format must be"))
		})

		It("rejects non-numeric ASN", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "asnX:10"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-0/type-2"))
		})

		It("rejects non-numeric assigned number", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "10:abc"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Assigned Number"))
		})

		It("aggregates multiple invalid route targets", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "10.0.0.1:70000"}, // type-1 overflow
				{Value: "0:10"},           // reserved ASN
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-1"))
			Expect(err.Error()).To(ContainSubstring("reserved"))
		})

		It("reports only invalid among mixed valid", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "192.0.2.1:10"},
				{Value: "65000:100"},
				{Value: "70000:70000"}, // invalid type-2 overflow
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type-2"))
			Expect(err.Error()).NotTo(ContainSubstring("192.0.2.1:10"))
		})

		It("accepts lower/upper boundaries", func() {
			obj.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{Value: "1:0"},
				{Value: "65534:4294967295"},
				{Value: "10.0.0.1:0"},
				{Value: "10.0.0.1:65535"},
				{Value: "65536:0"},
				{Value: "4294967294:65535"},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("ValidateUpdate RouteTargets", func() {
		It("allows unchanged valid route targets", func() {
			oldObj := &v1alpha1.VRF{Spec: v1alpha1.VRFSpec{RouteTargets: []v1alpha1.RouteTarget{{Value: "65000:10"}}}}
			newObj := oldObj.DeepCopy()
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects update introducing invalid route target", func() {
			oldObj := &v1alpha1.VRF{Spec: v1alpha1.VRFSpec{RouteTargets: []v1alpha1.RouteTarget{{Value: "65000:10"}}}}
			newObj := oldObj.DeepCopy()
			newObj.Spec.RouteTargets = append(newObj.Spec.RouteTargets, v1alpha1.RouteTarget{Value: "10.0.0.1:70000"})
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})
	})
})
