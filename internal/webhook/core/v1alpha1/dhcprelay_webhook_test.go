// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("DHCPRelay Webhook", func() {
	var (
		obj       *v1alpha1.DHCPRelay
		oldObj    *v1alpha1.DHCPRelay
		validator DHCPRelayCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.DHCPRelay{
			Spec: v1alpha1.DHCPRelaySpec{
				DeviceRef: v1alpha1.LocalObjectReference{Name: "leaf1"},
			},
		}
		oldObj = &v1alpha1.DHCPRelay{}
		validator = DHCPRelayCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("Deprecated InterfaceRefs field", func() {
		It("returns deprecation warning on create when InterfaceRefs is set", func() {
			obj.Spec.InterfaceRefs = []v1alpha1.LocalObjectReference{{Name: "eth0"}} //nolint:staticcheck
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("spec.interfaceRefs is deprecated")))
		})

		It("returns no warning on create when InterfaceRefs is not set", func() {
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("returns deprecation warning on update when InterfaceRefs is set", func() {
			newObj := obj.DeepCopy()
			newObj.Spec.InterfaceRefs = []v1alpha1.LocalObjectReference{{Name: "eth0"}} //nolint:staticcheck
			warnings, err := validator.ValidateUpdate(ctx, obj, newObj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("spec.interfaceRefs is deprecated")))
		})
	})
})
