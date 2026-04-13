// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("BGP Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind BGP")
			resource := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.10",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			var resource client.Object = &v1alpha1.BGP{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance BGP")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGP).To(BeNil(), "Provider should not have BGP instance configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGP).ToNot(BeNil(), "Provider should have BGP instance configured")
			}).Should(Succeed())
		})

		It("Should set ReadyCondition=False when vrfRef points to a non-existent VRF", func() {
			By("Updating the BGP to reference a non-existent VRF")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				resource.Spec.VrfRef = &v1alpha1.LocalObjectReference{Name: "does-not-exist"}
				g.Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			}).Should(Succeed())

			By("Expecting ReadyCondition to be False with WaitingForDependencies reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})

		It("Should pass VRF to the provider when vrfRef is set", func() {
			By("Creating a VRF on the same device")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-vrf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "CC-MGMT",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())
			})

			By("Updating the BGP to reference the VRF")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				resource.Spec.VrfRef = &v1alpha1.LocalObjectReference{Name: vrf.Name}
				g.Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			}).Should(Succeed())

			By("Ensuring the provider receives the VRF")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGPVRF).ToNot(BeNil())
				g.Expect(testProvider.BGPVRF.Spec.Name).To(Equal("CC-MGMT"))
			}).Should(Succeed())

			By("Ensuring ReadyCondition is True")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})
})
