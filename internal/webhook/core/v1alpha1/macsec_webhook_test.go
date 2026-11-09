// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("MacSec Webhook", func() {
	var (
		obj       *v1alpha1.MacSec
		oldObj    *v1alpha1.MacSec
		validator MacSecCustomValidator
	)

	BeforeEach(func() {
		now := time.Now()
		obj = &v1alpha1.MacSec{
			Name:      "test-macsec",
			Namespace: "default",
			Spec: v1alpha1.MacSecSpec{
				DeviceRef:    v1alpha1.LocalObjectReference{Name: "test-device"},
				Name:         "test-macsec",
				InterfaceRef: v1alpha1.LocalObjectReference{Name: "test-interface"},
				PreSharedKeys: []v1alpha1.PreSharedKey{
					{
						SecretRef:           v1alpha1.LocalObjectReference{Name: "test-secret"},
						ConnectivityKeyName: "abc123",
						Algorithm:           v1alpha1.AES256CMAC,
						StartTime:           metav1.NewTime(now),
						EndTime:             metav1.NewTime(now.Add(time.Hour)),
					},
				},
				Policy: &v1alpha1.MacSecPolicy{
					CipherSuite:           "GCM-AES-256",
					ConfidentialityOffset: v1alpha1.ZeroBytes,
					KeyServerPriority:     16,
					RelayProtection:       0,
				},
			},
		}
		oldObj = obj.DeepCopy()
		validator = MacSecCustomValidator{}
	})

	Context("When creating MacSec under Validating Webhook", func() {
		It("Should admit a valid MacSec", func() {
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit multiple valid pre-shared keys", func() {
			now := time.Now()
			obj.Spec.PreSharedKeys = append(obj.Spec.PreSharedKeys, v1alpha1.PreSharedKey{
				SecretRef:           v1alpha1.LocalObjectReference{Name: "test-secret-2"},
				ConnectivityKeyName: "def456",
				Algorithm:           v1alpha1.AES128CMAC,
				StartTime:           metav1.NewTime(now.Add(time.Hour)),
				EndTime:             metav1.NewTime(now.Add(2 * time.Hour)),
			})
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny a pre-shared key whose start time is after its end time", func() {
			now := time.Now()
			obj.Spec.PreSharedKeys[0].StartTime = metav1.NewTime(now.Add(2 * time.Hour))
			obj.Spec.PreSharedKeys[0].EndTime = metav1.NewTime(now.Add(time.Hour))
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start time"))
			Expect(err.Error()).To(ContainSubstring("after end time"))
		})

		It("Should deny a pre-shared key whose end time is in the past", func() {
			now := time.Now()
			obj.Spec.PreSharedKeys[0].StartTime = metav1.NewTime(now.Add(-2 * time.Hour))
			obj.Spec.PreSharedKeys[0].EndTime = metav1.NewTime(now.Add(-time.Hour))
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("end time"))
			Expect(err.Error()).To(ContainSubstring("not after current time"))
		})

		It("Should deny when one of several pre-shared keys is invalid", func() {
			now := time.Now()
			obj.Spec.PreSharedKeys = append(obj.Spec.PreSharedKeys, v1alpha1.PreSharedKey{
				SecretRef:           v1alpha1.LocalObjectReference{Name: "test-secret-2"},
				ConnectivityKeyName: "def456",
				Algorithm:           v1alpha1.AES128CMAC,
				StartTime:           metav1.NewTime(now.Add(time.Hour)),
				EndTime:             metav1.NewTime(now.Add(-time.Hour)),
			})
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When updating MacSec under Validating Webhook", func() {
		It("Should admit update with valid pre-shared keys", func() {
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny update introducing an invalid pre-shared key", func() {
			now := time.Now()
			obj.Spec.PreSharedKeys[0].StartTime = metav1.NewTime(now.Add(2 * time.Hour))
			obj.Spec.PreSharedKeys[0].EndTime = metav1.NewTime(now.Add(time.Hour))
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When deleting MacSec under Validating Webhook", func() {
		It("Should admit deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
