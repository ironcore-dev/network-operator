// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ = Describe("Device Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-device"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating the endpoint credentials as a Secret")
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, key, secret); errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Data: map[string][]byte{
						corev1.BasicAuthUsernameKey: []byte("user"),
						corev1.BasicAuthPasswordKey: []byte("password"),
					},
					Type: corev1.SecretTypeBasicAuth,
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				resource := &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: &v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
							SecretRef: &corev1.SecretReference{
								Name: name,
							},
						},
						Bootstrap: &v1alpha1.Bootstrap{
							Template: &v1alpha1.TemplateSource{
								Inline: ptr.To("device-template"),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, key, secret)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Secret")
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseActive))
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Manufacturer).To(Equal("Manufacturer"))
				g.Expect(resource.Status.Model).To(Equal("Model"))
				g.Expect(resource.Status.SerialNumber).To(Equal("123456789"))
				g.Expect(resource.Status.FirmwareVersion).To(Equal("1.0.0"))
			}).Should(Succeed())
		})
	})
})
