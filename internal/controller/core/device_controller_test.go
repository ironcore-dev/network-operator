// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
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
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
						SecretRef: &v1alpha1.SecretReference{
							Name: name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Verifying the device transitions to Active phase when no provisioning is configured")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning))
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.ReadyReason))
			}).Should(Succeed())

			By("Creating the custom resource for the Kind Interface")
			iface := &v1alpha1.Interface{}
			if err := k8sClient.Get(ctx, key, iface); errors.IsNotFound(err) {
				resource := &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
						Name:        "eth1/1",
						AdminState:  "Up",
						Description: "Test",
						MTU:         9000,
						Type:        "Physical",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning))

				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))

				g.Expect(resource.Status.Manufacturer).To(Equal("Manufacturer"))
				g.Expect(resource.Status.Model).To(Equal("Model"))
				g.Expect(resource.Status.SerialNumber).To(Equal("123456789"))
				g.Expect(resource.Status.FirmwareVersion).To(Equal("1.0.0"))

				g.Expect(resource.Status.Ports).To(HaveLen(8))
				g.Expect(resource.Status.Ports[0].Name).To(Equal("eth1/1"))
				g.Expect(resource.Status.Ports[0].Type).To(Equal("10g"))
				g.Expect(resource.Status.Ports[0].SupportedSpeedsGbps).To(Equal([]int32{1, 10}))
				g.Expect(resource.Status.Ports[0].Transceiver).To(Equal("QSFP-DD"))
				g.Expect(resource.Status.Ports[0].InterfaceRef).ToNot(BeNil())
				g.Expect(resource.Status.Ports[0].InterfaceRef.Name).To(Equal(name))
				g.Expect(resource.Status.PostSummary).To(Equal("1/8 (10g)"))
			}).Should(Succeed())

			By("Cleanup the specific resource instance Interface")
			intf := &v1alpha1.Interface{}
			err := k8sClient.Get(ctx, key, intf)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
		})

		It("Should transition from Pending to Provisioning when provisioning is configured", func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
						SecretRef: &v1alpha1.SecretReference{
							Name: name,
						},
					},
					Provisioning: &v1alpha1.Provisioning{
						Image: v1alpha1.Image{
							URL:          "http://example.com/nxos.bin",
							Checksum:     "d41d8cd98f00b204e9800998ecf8427e",
							ChecksumType: v1alpha1.ChecksumTypeMD5,
						},
						BootScript: v1alpha1.TemplateSource{
							Inline: ptr.To("boot nxos.bin"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Verifying the device transitions to Provisioning phase")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseProvisioning))
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.ProvisioningReason))
			}).Should(Succeed())
		})

		It("Should transition from ProvisioningCompleted to Active", func() {
			By("Creating a Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.5:9339",
						SecretRef: &v1alpha1.SecretReference{
							Name: name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Setting the device to Provisioned phase with provisioning info")
			Eventually(func(g Gomega) {
				resource := device.DeepCopy()
				resource.Status.Phase = v1alpha1.DevicePhaseProvisioned
				resource.Status.Provisioning = []v1alpha1.ProvisioningInfo{
					{
						Token:     "test-token",
						StartTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
					},
				}
				g.Expect(k8sClient.Status().Patch(ctx, resource, client.MergeFrom(device))).To(Succeed())
			}).Should(Succeed())

			By("Verifying the provisioning status has an end time set")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Provisioning).To(HaveLen(1))
				g.Expect(resource.Status.Provisioning[0].EndTime).ToNot(BeNil())
			}).Should(Succeed())

			By("Verifying the device transitions to Active phase")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning))
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.ReadyReason))
			}).Should(Succeed())
		})

		It("Should transition from Active to Provisioning once the reset-phase-to-provisioning annotation is set", func() {
			By("Creating a Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.5:9339",
						SecretRef: &v1alpha1.SecretReference{
							Name: name,
						},
					},
					Provisioning: &v1alpha1.Provisioning{
						BootScript: v1alpha1.TemplateSource{
							Inline: ptr.To("boot nxos.bin"),
						},
						Image: v1alpha1.Image{
							URL:          "https://best-vendor-images.to/windows98",
							Checksum:     "d41d8cd98f00b204e9800998ecf8427e",
							ChecksumType: v1alpha1.ChecksumTypeMD5,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Setting the device to Running phase")
			orig := device.DeepCopy()
			device.Status.Phase = v1alpha1.DevicePhaseRunning
			Expect(k8sClient.Status().Patch(ctx, device, client.MergeFrom(orig))).To(Succeed())

			By("Verifying the device transitions to Running phase")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning))
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
			}).Should(Succeed())

			By("Adding the reset-phase-to-provisioning annotation to the device")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				patch := resource.DeepCopy()
				annotations := make(map[string]string)
				annotations[v1alpha1.DeviceMaintenanceAnnotation] = v1alpha1.DeviceMaintenanceResetPhaseToProvisioning
				patch.SetAnnotations(annotations)
				g.Expect(k8sClient.Patch(ctx, patch, client.MergeFrom(resource))).To(Succeed())
			}).Should(Succeed())

			By("Verifying the device transitions to Provisioning phase and the annotation is removed")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Phase).To(Equal(v1alpha1.DevicePhaseProvisioning))
				_, exists := resource.Annotations[v1alpha1.DeviceMaintenanceAnnotation]
				g.Expect(exists).To(BeFalse(), "Maintenance annotation should be removed after processing")
			}).WithTimeout(time.Second * 10).Should(Succeed())
		})
	})
})
