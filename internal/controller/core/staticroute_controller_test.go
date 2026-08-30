// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("StaticRoute Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name    string
			key     client.ObjectKey
			vrfName string
		)

		BeforeEach(func() {
			By("Creating a test Device resource")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-staticroute-",
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

			By("Creating a test VRF resource")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-vrf-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       make(map[string]string),
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{
						Name: device.Name,
					},
					Name: "test-vrf",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			vrfName = vrf.Name
		})

		AfterEach(func() {
			By("Cleaning up all StaticRoute resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.StaticRoute{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleaning up test VRF resource")
			vrf := &v1alpha1.VRF{}
			vrfKey := client.ObjectKey{Name: vrfName, Namespace: metav1.NamespaceDefault}
			if err := k8sClient.Get(ctx, vrfKey, vrf); err == nil {
				Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())
			}

			By("Cleaning up test Device resource")
			device := &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying all StaticRoutes are deleted")
			Eventually(func(g Gomega) {
				srList := &v1alpha1.StaticRouteList{}
				g.Expect(k8sClient.List(ctx, srList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				g.Expect(srList.Items).To(BeEmpty())
			}).Should(Succeed())
		})

		It("Should successfully reconcile a StaticRoute resource", func() {
			By("Creating a StaticRoute with IPv4 routes")
			distance := int32(1)
			staticRoute := &v1alpha1.StaticRoute{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-staticroute-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.StaticRouteSpec{
					DeviceRef: v1alpha1.LocalObjectReference{
						Name: name,
					},
					Name: "test-static-route",
					VrfRef: &v1alpha1.LocalObjectReference{
						Name: vrfName,
					},
					Prefix: v1alpha1.IPPrefix{
						Prefix: netip.MustParsePrefix("10.0.0.0/24"),
					},
					NextHops: []*v1alpha1.NextHop{
						{
							Address: "192.168.1.1",
							Metric:  &distance,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, staticRoute)).To(Succeed())
			staticRouteKey := client.ObjectKeyFromObject(staticRoute)

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.ConfiguredReason))
			}).Should(Succeed())
		})

		It("Should handle StaticRoute with missing VRF reference", func() {
			By("Creating a StaticRoute referencing non-existent VRF")
			distance := int32(1)
			staticRoute := &v1alpha1.StaticRoute{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-staticroute-missing-vrf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.StaticRouteSpec{
					DeviceRef: v1alpha1.LocalObjectReference{
						Name: name,
					},
					Name: "test-static-route-missing-vrf",
					VrfRef: &v1alpha1.LocalObjectReference{
						Name: "non-existent-vrf",
					},
					Prefix: v1alpha1.IPPrefix{
						Prefix: netip.MustParsePrefix("172.16.0.0/12"),
					},
					NextHops: []*v1alpha1.NextHop{
						{
							Address: "10.0.0.254",
							Metric:  &distance,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, staticRoute)).To(Succeed())
			staticRouteKey := client.ObjectKeyFromObject(staticRoute)

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).NotTo(BeEmpty())

				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.VRFNotFoundReason))
				g.Expect(cond.Message).To(ContainSubstring("non-existent-vrf"))
			}).Should(Succeed())

			By("Verifying ReadyCondition is False")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.StaticRoute{}
				g.Expect(k8sClient.Get(ctx, staticRouteKey, resource)).To(Succeed())
				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})
	})
})
