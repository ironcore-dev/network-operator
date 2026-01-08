// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("PrefixSet Webhook", func() {
	var (
		obj       *v1alpha1.PrefixSet
		oldObj    *v1alpha1.PrefixSet
		validator PrefixSetCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.PrefixSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-prefix-set",
				Namespace: "default",
			},
			Spec: v1alpha1.PrefixSetSpec{
				DeviceRef: v1alpha1.LocalObjectReference{Name: "test-device"},
				Name:      "TEST",
			},
		}
		oldObj = obj.DeepCopy()
		validator = PrefixSetCustomValidator{}
	})

	Describe("ValidateCreate", func() {
		It("should allow creation with valid IPv4 prefixes", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.1.0/24"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("10.0.0.0/8"),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow creation with valid IPv6 prefixes", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("2001:db8::/32"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("fe80::/10"),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow creation with mask length ranges", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/16"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 16,
						Max: 24,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject creation with mixed IP families", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.1.0/24"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("2001:db8::/32"),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject creation with invalid mask length range min less than prefix bits", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/24"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 16, // Less than prefix bits (24)
						Max: 28,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject creation with invalid mask length range max less than prefix bits", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/24"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 24,
						Max: 20, // Less than prefix bits (24)
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject creation with min greater than max in mask length range", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/16"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 28,
						Max: 24, // Min > Max
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject creation with IPv4 mask length exceeding 32 bits", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/16"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 16,
						Max: 64, // Exceeds IPv4 maximum of 32
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ValidateUpdate", func() {
		It("should allow update with valid IPv4 prefixes", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.2.0/24"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("10.1.0.0/16"),
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow update with valid IPv6 prefixes", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("2001:db8:1::/48"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("fe80::/64"),
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject update with IP family change", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.2.0/24"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("10.1.0.0/16"),
				},
			}
			oldObj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("2001:db8:1::/48"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("fe80::/64"),
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject update with mixed IP families", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.1.0/24"),
				},
				{
					Sequence: 20,
					Prefix:   v1alpha1.MustParsePrefix("2001:db8::/32"),
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject update with invalid mask length ranges", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("192.168.0.0/24"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 32,
						Max: 28, // Min > Max
					},
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("should reject update with IPv4 mask length exceeding 32 bits", func() {
			obj.Spec.Entries = []v1alpha1.PrefixEntry{
				{
					Sequence: 10,
					Prefix:   v1alpha1.MustParsePrefix("10.0.0.0/8"),
					MaskLengthRange: &v1alpha1.MaskLengthRange{
						Min: 8,
						Max: 64, // Exceeds IPv4 maximum of 32
					},
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ValidateDelete", func() {
		It("should always allow deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
