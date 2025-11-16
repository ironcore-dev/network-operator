// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("Interface Webhook", func() {
	var (
		obj       *v1alpha1.Interface
		oldObj    *v1alpha1.Interface
		validator InterfaceCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				DeviceRef:  v1alpha1.LocalObjectReference{Name: "test-device"},
				Name:       "test-interface",
				AdminState: v1alpha1.AdminStateUp,
				Type:       v1alpha1.InterfaceTypeLoopback,
			},
		}
		oldObj = &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				DeviceRef:  v1alpha1.LocalObjectReference{Name: "test-device"},
				Name:       "test-interface",
				AdminState: v1alpha1.AdminStateUp,
				Type:       v1alpha1.InterfaceTypeLoopback,
			},
		}
		validator = InterfaceCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("When creating or updating Interfaces under Validating Webhook", func() {
		It("Should allow valid IPv4 addresses", func() {
			obj.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
				Addresses: []v1alpha1.IPPrefix{
					{Prefix: netip.MustParsePrefix("10.0.0.1/32")},
					{Prefix: netip.MustParsePrefix("10.0.1.1/32")},
				},
			}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject IPv6 addresses in IPv4 field", func() {
			obj.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
				Addresses: []v1alpha1.IPPrefix{
					{Prefix: netip.MustParsePrefix("2001:db8::1/128")},
				},
			}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid IPv4 address"))
			Expect(err.Error()).To(ContainSubstring("address is IPv6"))
		})

		It("Should reject overlapping IPv4 addresses", func() {
			obj.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
				Addresses: []v1alpha1.IPPrefix{
					{Prefix: netip.MustParsePrefix("10.0.0.0/24")},
					{Prefix: netip.MustParsePrefix("10.0.0.128/25")},
				},
			}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps with"))
		})

		It("Should reject updates with invalid IPv4 addresses", func() {
			obj.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
				Addresses: []v1alpha1.IPPrefix{
					{Prefix: netip.MustParsePrefix("10.0.0.0/24")},
					{Prefix: netip.MustParsePrefix("10.0.0.128/25")},
				},
			}
			_, err := validator.ValidateUpdate(context.Background(), oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps with"))
		})
	})
})
