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

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
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

		It("Should successfully reconcile with valid ProviderConfigRef", func() {
			const (
				interfaceName  = "testlldp-provider-interface"
				lldpConfigName = "testlldp-provider-config"
			)

			By("Creating the Interface resource")
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

			By("Creating the LLDPConfig resource")
			lldpConfig := &nxv1alpha1.LLDPConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lldpConfigName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: nxv1alpha1.LLDPConfigSpec{
					InitDelay: 5,
					HoldTime:  180,
					InterfaceRefs: []nxv1alpha1.LLDPInterface{{
						LocalObjectReference: v1alpha1.LocalObjectReference{Name: interfaceName},
						AdminRxState:         v1alpha1.AdminStateUp,
						AdminTxState:         v1alpha1.AdminStateDown,
					}},
				},
			}
			Expect(k8sClient.Create(ctx, lldpConfig)).To(Succeed())

			By("Creating LLDP with valid ProviderConfigRef")
			lldp = &v1alpha1.LLDP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.LLDPSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					AdminState: v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: nxv1alpha1.GroupVersion.String(),
						Kind:       "LLDPConfig",
						Name:       lldpConfigName,
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

				cond = meta.FindStatusCondition(lldp.Status.Conditions, v1alpha1.OperationalCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the LLDP is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.LLDP).ToNot(BeNil())
				g.Expect(testProvider.LLDP.GetName()).To(Equal(resourceName))
			}).Should(Succeed())

			By("Cleaning up the LLDPConfig resource")
			Expect(k8sClient.Delete(ctx, lldpConfig)).To(Succeed())

			By("Cleaning up the Interface resource")
			Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
		})
	})
})
