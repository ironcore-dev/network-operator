// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("CommunitySet Controller", func() {
	Context("When reconciling a resource", func() {
		const set = "BGP-COMMUNITY"
		var (
			name   string
			key    client.ObjectKey
			device *v1alpha1.Device
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{
				GenerateName: "test-communityset-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind CommunitySet")
			resource := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      set,
					Type:      v1alpha1.CommunitySetTypeStandard,
					Members: []v1alpha1.CommunityMember{
						{Sequence: 5, Regex: "65000:[0-9]+"},
						{Sequence: 10, Regex: "65001:[0-9]+"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the CommunitySet resource")
			cs := &v1alpha1.CommunitySet{}
			cs.Name = name
			cs.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cs))).To(Succeed())

			By("Verifying the resource is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(device.Name).CommunitySets.Has(set)).To(BeFalse(), "Provider should not have CommunitySet configured")
			}).Should(Succeed())

			By("Cleaning up the Device resource")
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.CommunitySet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.CommunitySet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as an owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.CommunitySet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.CommunitySet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(device.Name).CommunitySets.Has(set)).To(BeTrue(), "Provider should have CommunitySet configured")
			}).Should(Succeed())
		})
	})
})
