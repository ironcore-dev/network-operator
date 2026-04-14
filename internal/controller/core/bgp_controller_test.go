// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

var _ = Describe("BGP Controller", func() {
	Context("When reconciling a resource", func() {
		var device *v1alpha1.Device

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device = &v1alpha1.Device{
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
		})

		AfterEach(func() {
			By("Deleting the Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind BGP")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.10",
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
				Eventually(func(g Gomega) {
					b := &v1alpha1.BGP{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b))).To(BeTrue())
				}).Should(Succeed())
				By("Ensuring the resource is deleted from the provider")
				Eventually(func(g Gomega) {
					g.Expect(testProvider.BGP).To(BeNil(), "Provider should not have BGP instance configured")
				}).Should(Succeed())
			})

			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, device.Name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(device.Name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
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
			By("Creating a BGP with a vrfRef pointing to a non-existent VRF")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.11",
					VrfRef:    &v1alpha1.LocalObjectReference{Name: "does-not-exist"},
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
				Eventually(func(g Gomega) {
					b := &v1alpha1.BGP{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Expecting ReadyCondition to be False with VRFNotFoundReason reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				cond := conditions.Get(resource, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.VRFNotFoundReason))
			}).Should(Succeed())
		})

		It("Should pass VRF to the provider when vrfRef is set", func() {
			By("Creating a VRF")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-vrf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Name:      "CC-MGMT",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())
			})

			By("Creating a BGP with the vrfRef set")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.12",
					VrfRef:    &v1alpha1.LocalObjectReference{Name: vrf.Name},
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
				Eventually(func(g Gomega) {
					b := &v1alpha1.BGP{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Ensuring the provider receives the VRF")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGPVRF).ToNot(BeNil())
				g.Expect(testProvider.BGPVRF.Spec.Name).To(Equal("CC-MGMT"))
			}).Should(Succeed())

			By("Ensuring ReadyCondition is True")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				cond := conditions.Get(resource, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		It("Should reconcile BGP when a referenced RoutingPolicy is created", func() {
			By("Creating a BGP with a redistributeDirectRoutes ref pointing to a non-existent RoutingPolicy")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.20",
					AddressFamilies: &v1alpha1.BGPAddressFamilies{
						Ipv4Unicast: &v1alpha1.BGPUnicastAddressFamily{
							BGPAddressFamily: v1alpha1.BGPAddressFamily{Enabled: true},
							RedistributeDirectRoutes: &v1alpha1.BGPRedistributeDirectRoutes{
								RoutingPolicyRef: v1alpha1.LocalObjectReference{Name: "test-policy"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
				Eventually(func(g Gomega) {
					b := &v1alpha1.BGP{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Expecting ReadyCondition to be False with WaitingForDependencies reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				cond := conditions.Get(resource, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())

			By("Creating the RoutingPolicy")
			rp := &v1alpha1.RoutingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Name:      "test-policy",
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, rp)).To(Succeed())
				Eventually(func(g Gomega) {
					r := &v1alpha1.RoutingPolicy{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(rp), r))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Expecting ReadyCondition to become True after the RoutingPolicy is created")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				cond := conditions.Get(resource, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		It("Should reject VrfRef changes via the API server", func() {
			By("Creating the custom resource for the Kind BGP")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.10",
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
				Eventually(func(g Gomega) {
					b := &v1alpha1.BGP{}
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Waiting for the BGP to be reconciled so we know it exists")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Attempting to set VrfRef (must be rejected — immutable)")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), resource)).To(Succeed())
				resource.Spec.VrfRef = &v1alpha1.LocalObjectReference{Name: "some-vrf"}
				err := k8sClient.Update(ctx, resource)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("VrfRef is immutable"))
			}).Should(Succeed())
		})
	})
})
