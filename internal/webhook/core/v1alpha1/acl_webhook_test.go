// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("AccessControlList Webhook", func() {
	var (
		obj       *v1alpha1.AccessControlList
		oldObj    *v1alpha1.AccessControlList
		validator AccessControlListCustomValidator
	)

	BeforeEach(func() {
		obj = &v1alpha1.AccessControlList{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-acl",
				Namespace: "default",
			},
			Spec: v1alpha1.AccessControlListSpec{
				DeviceRef: v1alpha1.LocalObjectReference{Name: "test-device"},
				Name:      "test-acl",
				Entries: []v1alpha1.ACLEntry{
					{
						Sequence:           1,
						Action:             v1alpha1.ActionPermit,
						Protocol:           v1alpha1.ProtocolIP,
						SourceAddress:      v1alpha1.MustParsePrefix("10.0.0.0/8"),
						DestinationAddress: v1alpha1.MustParsePrefix("192.168.0.0/16"),
					},
				},
			},
		}
		oldObj = obj.DeepCopy()
		validator = AccessControlListCustomValidator{}
	})

	Context("When creating AccessControlList under Validating Webhook", func() {
		It("Should admit a valid IPv4-only ACL", func() {
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit a valid IPv6-only ACL", func() {
			obj.Spec.Entries = []v1alpha1.ACLEntry{
				{
					Sequence:           1,
					Action:             v1alpha1.ActionDeny,
					Protocol:           v1alpha1.ProtocolTCP,
					SourceAddress:      v1alpha1.MustParsePrefix("2001:db8::/32"),
					DestinationAddress: v1alpha1.MustParsePrefix("fd00::/8"),
				},
				{
					Sequence:           2,
					Action:             v1alpha1.ActionPermit,
					Protocol:           v1alpha1.ProtocolIP,
					SourceAddress:      v1alpha1.MustParsePrefix("::/0"),
					DestinationAddress: v1alpha1.MustParsePrefix("::/0"),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny an entry with mixed source (IPv4) and destination (IPv6)", func() {
			obj.Spec.Entries[0].SourceAddress = v1alpha1.MustParsePrefix("10.0.0.0/8")
			obj.Spec.Entries[0].DestinationAddress = v1alpha1.MustParsePrefix("2001:db8::/32")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("entries[0]"))
			Expect(err.Error()).To(ContainSubstring("same IP address family"))
		})

		It("Should deny an entry with mixed source (IPv6) and destination (IPv4)", func() {
			obj.Spec.Entries[0].SourceAddress = v1alpha1.MustParsePrefix("2001:db8::/32")
			obj.Spec.Entries[0].DestinationAddress = v1alpha1.MustParsePrefix("10.0.0.0/8")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("entries[0]"))
			Expect(err.Error()).To(ContainSubstring("same IP address family"))
		})

		It("Should deny entries mixing IPv4 and IPv6 across entries", func() {
			obj.Spec.Entries = []v1alpha1.ACLEntry{
				{
					Sequence:           1,
					Action:             v1alpha1.ActionPermit,
					Protocol:           v1alpha1.ProtocolIP,
					SourceAddress:      v1alpha1.MustParsePrefix("10.0.0.0/8"),
					DestinationAddress: v1alpha1.MustParsePrefix("0.0.0.0/0"),
				},
				{
					Sequence:           2,
					Action:             v1alpha1.ActionDeny,
					Protocol:           v1alpha1.ProtocolIP,
					SourceAddress:      v1alpha1.MustParsePrefix("2001:db8::/32"),
					DestinationAddress: v1alpha1.MustParsePrefix("::/0"),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("entries[1]"))
			Expect(err.Error()).To(ContainSubstring("same IP address family"))
		})
	})

	Context("When updating AccessControlList under Validating Webhook", func() {
		It("Should admit update with consistent IP families", func() {
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny update introducing mixed families", func() {
			obj.Spec.Entries[0].DestinationAddress = v1alpha1.MustParsePrefix("2001:db8::/32")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When deleting AccessControlList under Validating Webhook", func() {
		It("Should admit deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
