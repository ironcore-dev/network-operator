// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ = Describe("Interface Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-interface"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device := &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				resource := &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up all Interface resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Interface{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Verifying the Interface is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeFalse(), "Provider shouldn't have Interface configured anymore")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a Physical Interface with IPv4 addresses", func() {
			By("Creating an Interface with IPv4 addresses")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have Interface configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a Physical Interface with unnumbered IPv4", func() {
			By("Creating a Loopback Interface with IPv4 addresses")
			lb := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-lb",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, lb)).To(Succeed())

			By("Creating a Physical Interface with unnumbered IPv4 configuration")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: lb.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		It("Should handle unnumbered reference to Interface from different device", func() {
			By("Creating a Loopback Interface on a different device")
			lb := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-lb",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "non-existing-device"},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, lb)).To(Succeed())

			By("Creating a Physical Interface with unnumbered reference to the cross-device Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: lb.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets invalid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.UnnumberedCrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
			}).Should(Succeed())
		})

		It("Should handle unnumbered reference to non-existing Interface", func() {
			By("Creating a Physical Interface with unnumbered reference to non-existing Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: "non-existing-interface"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets Interface not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.UnnumberedSourceInterfaceNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
			}).Should(Succeed())
		})

		It("Should handle unnumbered reference to non-Loopback Interface", func() {
			By("Creating a Physical Interface to be referenced")
			phys := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-phys",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name + "-phys",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.2/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, phys)).To(Succeed())

			By("Creating a Physical Interface with unnumbered reference to the Physical Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: phys.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets invalid interface type status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.UnnumberedInvalidInterfaceTypeReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
			}).Should(Succeed())
		})
	})
})
