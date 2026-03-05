// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("LLDP Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			deviceName   = "testlldp-device"
			resourceName = "testlldp-lldp"
		)

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			lldp   *v1alpha1.LLDP
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp = &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

				By("Waiting for LLDP resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind LLDP")
			lldp = &v1alpha1.LLDP{}
			if err := k8sClient.Get(ctx, resourceKey, lldp); errors.IsNotFound(err) {
				lldp = &v1alpha1.LLDP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.LLDPSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, lldp)).To(Succeed())
			}

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(lldp, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(lldp.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, deviceName))
			}).Should(Succeed())

			By("Verifying the controller sets the owner reference")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(lldp.OwnerReferences).To(HaveLen(1))
				g.Expect(lldp.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(lldp.OwnerReferences[0].Name).To(Equal(deviceName))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(lldp.Status.Conditions).To(HaveLen(3))

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				cond = meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				cond = meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.OperationalCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the LLDP is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil(), "Provider LLDP should not be nil")
				if testProvider.LLDP != nil {
					g.Expect(testProvider.LLDP.GetName()).To(Equal(resourceName), "Provider should have LLDP configured")
				}
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource with AdminState Down", func() {
			By("Creating the custom resource for the Kind LLDP with AdminState Down")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateDown,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(lldp, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				g.Expect(lldp.Status.Conditions).To(HaveLen(3))

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the LLDP is created in the provider with AdminState Down")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				if testProvider.LLDP != nil {
					g.Expect(testProvider.LLDP.Spec.AdminState).To(Equal(v1alpha1.AdminStateDown))
				}
			}).Should(Succeed())
		})

		It("Should reject duplicate LLDP resources on the same device", func() {
			By("Creating the first LLDP resource")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for the first LLDP to be ready")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating a second LLDP resource for the same device")
			duplicateName := resourceName + "-duplicate"
			duplicateKey := client.ObjectKey{Name: duplicateName, Namespace: metav1.NamespaceDefault}
			duplicateLLDP := &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      duplicateName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, duplicateLLDP)).To(Succeed())

			By("Verifying the second LLDP has a ConfiguredCondition=False with DuplicateResourceOnDevice reason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, duplicateKey, duplicateLLDP)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(duplicateLLDP.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.DuplicateResourceOnDevice))
			}).Should(Succeed())

			By("Cleaning up the duplicate LLDP resource")
			Expect(k8sClient.Delete(ctx, duplicateLLDP)).To(Succeed())
		})

		It("Should properly handle deletion and cleanup", func() {
			By("Creating the custom resource for the Kind LLDP")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for the LLDP to be ready")
			Eventually(func(g Gomega) {
				lldp = &v1alpha1.LLDP{}
				g.Expect(k8sClient.Get(ctx, resourceKey, lldp)).To(Succeed())
				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying LLDP is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
			}).Should(Succeed())

			By("Deleting the LLDP resource")
			Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

			By("Verifying the LLDP is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured after deletion")
			}).Should(Succeed())

			By("Verifying the resource is fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	Context("When DeviceRef references non-existent Device", func() {
		const resourceName = "testlldp-nodevice-lldp"

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp := &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				// Remove finalizer if present to allow deletion
				if controllerutil.ContainsFinalizer(lldp, v1alpha1.FinalizerName) {
					controllerutil.RemoveFinalizer(lldp, v1alpha1.FinalizerName)
					Expect(k8sClient.Update(ctx, lldp)).To(Succeed())
				}
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())
			}
		})

		It("Should not add finalizer when Device does not exist", func() {
			By("Creating LLDP referencing a non-existent Device")
			lldp := &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "non-existent-device"},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller does not add a finalizer")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(controllerutil.ContainsFinalizer(lldp, v1alpha1.FinalizerName)).To(BeFalse())
			}).Should(Succeed())
		})
	})

	Context("When Device is paused", func() {
		const (
			deviceName   = "testlldp-paused-device"
			resourceName = "testlldp-paused-lldp"
		)

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			lldp   *v1alpha1.LLDP
		)

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.6:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp = &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

				By("Waiting for LLDP resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			device = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				// Ensure device is not paused before deletion
				if device.Spec.Paused != nil && *device.Spec.Paused {
					device.Spec.Paused = nil
					Expect(k8sClient.Update(ctx, device)).To(Succeed())
				}
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying the provider has been cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured")
			}).Should(Succeed())
		})

		It("Should skip reconciliation when Device is paused", func() {
			By("Creating LLDP resource")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for LLDP to be ready")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Pausing the Device")
			paused := true
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, deviceKey, device)
				g.Expect(err).NotTo(HaveOccurred())
				device.Spec.Paused = &paused
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			By("Updating LLDP AdminState to Down")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())
				lldp.Spec.AdminState = v1alpha1.AdminStateDown
				g.Expect(k8sClient.Update(ctx, lldp)).To(Succeed())
			}).Should(Succeed())

			By("Verifying the provider still has AdminState Up (reconciliation was skipped)")
			Consistently(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				g.Expect(testProvider.LLDP.Spec.AdminState).To(Equal(v1alpha1.AdminStateUp))
			}).Should(Succeed())

			By("Unpausing the Device")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, deviceKey, device)
				g.Expect(err).NotTo(HaveOccurred())
				device.Spec.Paused = nil
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			By("Verifying the provider now has AdminState Down (reconciliation resumed)")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				g.Expect(testProvider.LLDP.Spec.AdminState).To(Equal(v1alpha1.AdminStateDown))
			}).Should(Succeed())
		})
	})

	Context("When reconciling with ProviderConfigRef", func() {
		const (
			deviceName   = "testlldp-provider-device"
			resourceName = "testlldp-provider-lldp"
		)

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			lldp   *v1alpha1.LLDP
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp = &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

				By("Waiting for LLDP resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured")
			}).Should(Succeed())
		})

		It("Should handle missing ProviderConfigRef", func() {
			By("Creating LLDP with a non-existent ProviderConfigRef")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: "nx.cisco.networking.metal.ironcore.dev/v1alpha1",
						Kind:       "LLDPConfig",
						Name:       "non-existent-config",
					},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})

		It("Should handle invalid ProviderConfigRef API version", func() {
			By("Creating LLDP with invalid API version in ProviderConfigRef")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: "invalid-api-version",
						Kind:       "LLDPConfig",
						Name:       "some-config",
					},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with IncompatibleProviderConfigRef")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})

		It("Should handle unsupported ProviderConfigRef Kind", func() {
			By("Creating LLDP with unsupported Kind in ProviderConfigRef")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       "some-config",
					},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with IncompatibleProviderConfigRef")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})

	})

	Context("When reconciling with InterfaceRefs", func() {
		const (
			deviceName   = "testlldp-intfref-device"
			resourceName = "testlldp-intfref-lldp"
		)

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			lldp   *v1alpha1.LLDP
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.3:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp = &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

				By("Waiting for LLDP resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured")
			}).Should(Succeed())
		})

		It("Should handle missing InterfaceRef", func() {
			By("Creating LLDP with a non-existent InterfaceRef")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					InterfaceRefs: []v1alpha1.LLDPInterface{{
						LocalObjectReference: v1alpha1.LocalObjectReference{Name: "non-existent-interface"},
						AdminState:           v1alpha1.AdminStateUp,
					}},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})

		It("Should handle InterfaceRef belonging to a different device", func() {
			const (
				otherDeviceName    = "testlldp-other-device"
				otherInterfaceName = "testlldp-other-interface"
			)

			By("Creating another Device")
			otherDevice := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherDeviceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.99:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())

			By("Creating an Interface on the other Device")
			otherInterface := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherInterfaceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: otherDeviceName},
					Name:       "Ethernet1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, otherInterface)).To(Succeed())

			By("Creating LLDP referencing an Interface from a different device")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					InterfaceRefs: []v1alpha1.LLDPInterface{{
						LocalObjectReference: v1alpha1.LocalObjectReference{Name: otherInterfaceName},
						AdminState:           v1alpha1.AdminStateUp,
					}},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with CrossDeviceReferenceReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())

			By("Cleaning up the other Interface and Device")
			Expect(k8sClient.Delete(ctx, otherInterface)).To(Succeed())
			Expect(k8sClient.Delete(ctx, otherDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
		})

		It("Should successfully reconcile with multiple InterfaceRefs", func() {
			const (
				interface1Name = "testlldp-intfref-intf1"
				interface2Name = "testlldp-intfref-intf2"
			)

			By("Creating the first Interface")
			intf1 := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      interface1Name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, intf1)).To(Succeed())

			By("Creating the second Interface")
			intf2 := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      interface2Name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/2",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, intf2)).To(Succeed())

			By("Creating LLDP with multiple InterfaceRefs")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					InterfaceRefs: []v1alpha1.LLDPInterface{
						{
							LocalObjectReference: v1alpha1.LocalObjectReference{Name: interface1Name},
							AdminState:           v1alpha1.AdminStateUp,
						},
						{
							LocalObjectReference: v1alpha1.LocalObjectReference{Name: interface2Name},
							AdminState:           v1alpha1.AdminStateDown,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Verifying the controller sets all conditions to True")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				cond = meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Cleaning up the Interface resources")
			Expect(k8sClient.Delete(ctx, intf1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, intf2)).To(Succeed())
		})
	})

	Context("When updating LLDP spec", func() {
		const (
			deviceName   = "testlldp-update-device"
			resourceName = "testlldp-update-lldp"
		)

		resourceKey := client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			lldp   *v1alpha1.LLDP
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.4:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the LLDP resource")
			lldp = &v1alpha1.LLDP{}
			err := k8sClient.Get(ctx, resourceKey, lldp)
			if err == nil {
				Expect(k8sClient.Delete(ctx, lldp)).To(Succeed())

				By("Waiting for LLDP resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.LLDP{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).To(BeNil(), "Provider should have no LLDP configured")
			}).Should(Succeed())
		})

		It("Should handle AdminState update from Up to Down", func() {
			By("Creating LLDP with AdminState Up")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for LLDP to be ready")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying provider has AdminState Up")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				g.Expect(testProvider.LLDP.Spec.AdminState).To(Equal(v1alpha1.AdminStateUp))
			}).Should(Succeed())

			By("Updating AdminState to Down")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())
				lldp.Spec.AdminState = v1alpha1.AdminStateDown
				g.Expect(k8sClient.Update(ctx, lldp)).To(Succeed())
			}).Should(Succeed())

			By("Verifying provider has AdminState Down")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				g.Expect(testProvider.LLDP.Spec.AdminState).To(Equal(v1alpha1.AdminStateDown))
			}).Should(Succeed())
		})

		It("Should handle adding InterfaceRefs after creation", func() {
			const interfaceName = "testlldp-update-interface"

			By("Creating an Interface")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      interfaceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating LLDP without InterfaceRefs")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for LLDP to be ready")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Adding InterfaceRefs to LLDP")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())
				lldp.Spec.InterfaceRefs = []v1alpha1.LLDPInterface{{
					LocalObjectReference: v1alpha1.LocalObjectReference{Name: interfaceName},
					AdminState:           v1alpha1.AdminStateUp,
				}}
				g.Expect(k8sClient.Update(ctx, lldp)).To(Succeed())
			}).Should(Succeed())

			By("Verifying LLDP remains Ready after adding InterfaceRefs")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				g.Expect(lldp.Spec.InterfaceRefs).To(HaveLen(1))
			}).Should(Succeed())

			By("Cleaning up the Interface resource")
			Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
		})

		It("Should handle removing InterfaceRefs after creation", func() {
			const interfaceName = "testlldp-remove-interface"

			By("Creating an Interface")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      interfaceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating LLDP with InterfaceRefs")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					InterfaceRefs: []v1alpha1.LLDPInterface{{
						LocalObjectReference: v1alpha1.LocalObjectReference{Name: interfaceName},
						AdminState:           v1alpha1.AdminStateUp,
					}},
				},
			}
			Expect(k8sClient.Create(ctx, lldp)).To(Succeed())

			By("Waiting for LLDP to be ready")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				g.Expect(lldp.Spec.InterfaceRefs).To(HaveLen(1))
			}).Should(Succeed())

			By("Removing InterfaceRefs from LLDP")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())
				lldp.Spec.InterfaceRefs = nil
				g.Expect(k8sClient.Update(ctx, lldp)).To(Succeed())
			}).Should(Succeed())

			By("Verifying LLDP remains Ready after removing InterfaceRefs")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, lldp)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

				g.Expect(lldp.Spec.InterfaceRefs).To(BeEmpty())
			}).Should(Succeed())

			By("Cleaning up the Interface resource")
			Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
		})
	})
})
