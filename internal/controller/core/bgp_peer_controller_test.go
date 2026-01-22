// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

var _ = Describe("BGPPeer Controller", func() {
	Context("When reconciling a resource", func() {
		const host = "10.0.0.1"
		var device *v1alpha1.Device

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
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

		It("Should successfully reconcile a BGP peer", func() {
			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
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
			})

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				b := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b)).To(Succeed())
				g.Expect(conditions.IsReady(b)).To(BeTrue())
			}).Should(Succeed())

			By("Creating a BGPPeer resource")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: bgp.Name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
				By("Verifying the BGP peer is removed from the provider")
				Eventually(func(g Gomega) {
					g.Expect(testProvider.BGPPeers.Has(host)).To(BeFalse(), "Provider should not have BGP peer configured")
				}).Should(Succeed())
			})

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, device.Name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(device.Name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the BGP peer is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGPPeers.Has(host)).To(BeTrue(), "Provider should have BGP peer configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a BGP peer with local address", func() {
			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
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
			})

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				b := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b)).To(Succeed())
				g.Expect(conditions.IsReady(b)).To(BeTrue())
			}).Should(Succeed())

			By("Creating a Loopback Interface resource on the same device")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: device.Name},
					Name:       "Loopback0",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
			})

			By("Creating a BGPPeer resource with LocalAddress pointing to the Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: bgp.Name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			})

			By("Verifying the controller updates the status conditions successfully")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the BGP peer is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGPPeers.Has(host)).To(BeTrue(), "Provider should have BGP peer configured")
			}).Should(Succeed())
		})

		It("Should handle local address reference to non-existing Interface", func() {
			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
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
			})

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				b := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b)).To(Succeed())
				g.Expect(conditions.IsReady(b)).To(BeTrue())
			}).Should(Succeed())

			By("Creating a BGPPeer resource with LocalAddress pointing to a non-existent Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: bgp.Name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: "non-existing-interface"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			})

			By("Verifying the controller sets Interface not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InterfaceNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should reject local address reference to Interface on different device", func() {
			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
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
			})

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				b := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b)).To(Succeed())
				g.Expect(conditions.IsReady(b)).To(BeTrue())
			}).Should(Succeed())

			By("Creating a Loopback Interface resource on a different device")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:       "Loopback0",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
			})

			By("Creating a BGPPeer resource with LocalAddress pointing to the cross-device Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: bgp.Name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			})

			By("Verifying the BGP peer rejects the cross-device interface reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should set Configured=False with BGPNotFoundReason when bgpRef points to a non-existent BGP", func() {
			By("Creating a BGPPeer with a non-existent bgpRef")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: "does-not-exist"},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			})

			By("Verifying the controller sets ConfiguredCondition to False with BGPNotFoundReason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.BGPNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the BGP peer is NOT configured in the provider")
			Consistently(func(g Gomega) {
				g.Expect(testProvider.BGPPeers.Has(host)).To(BeFalse(), "Provider should not have BGP peer configured")
			}).Should(Succeed())
		})

		It("Should set Configured=False with WaitingForDependenciesReason when BGP exists but is not configured", func() {
			By("Creating a paused BGP resource (will not be configured)")
			pausedBGP := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-paused-bgp-",
					Namespace:    metav1.NamespaceDefault,
					Annotations: map[string]string{
						v1alpha1.PausedAnnotation: "true",
					},
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.11",
				},
			}
			Expect(k8sClient.Create(ctx, pausedBGP)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, pausedBGP)).To(Succeed())
			})

			By("Creating a BGPPeer referencing the paused BGP")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:    v1alpha1.LocalObjectReference{Name: pausedBGP.Name},
					Address:   "10.0.0.3",
					ASNumber:  intstr.FromInt(65002),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			})

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the BGP peer is NOT configured in the provider")
			Consistently(func(g Gomega) {
				g.Expect(testProvider.BGPPeers.Has("10.0.0.3")).To(BeFalse(), "Provider should not have BGP peer configured")
			}).Should(Succeed())
		})

		It("Should not reconcile iBGP peer if local-as is set", func() {
			By("Creating a BGP resource for the Device")
			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
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
			})

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				b := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgp), b)).To(Succeed())
				g.Expect(conditions.IsReady(b)).To(BeTrue())
			}).Should(Succeed())

			By("Creating a BGPPeer resource")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef:     v1alpha1.LocalObjectReference{Name: device.Name},
					BgpRef:        v1alpha1.LocalObjectReference{Name: bgp.Name},
					Address:       host,
					ASNumber:      intstr.FromInt(65000),
					LocalASNumber: &intstr.IntOrString{IntVal: 65000},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
				By("Verifying the BGP peer is removed from the provider")
				Eventually(func(g Gomega) {
					g.Expect(testProvider.BGPPeers.Has(host)).To(BeFalse(), "Provider should not have BGP peer configured")
				}).Should(Succeed())
			})

			By("Verifying the controller sets Configured=False with appropriate reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bgppeer), resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.ErrorReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
			}).Should(Succeed())
		})
	})
})
