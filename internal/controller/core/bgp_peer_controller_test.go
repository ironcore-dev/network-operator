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
		var (
			name   string
			key    client.ObjectKey
			bgpKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device := &v1alpha1.Device{
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
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating a BGP resource for the Device")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-bgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.10",
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())
			bgpKey = client.ObjectKey{Name: bgp.Name, Namespace: metav1.NamespaceDefault}

			By("Waiting for the BGP resource to be fully configured")
			Eventually(func(g Gomega) {
				bgp := &v1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, bgpKey, bgp)).To(Succeed())
				g.Expect(conditions.IsReady(bgp)).To(BeTrue())
			}).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up all BGPPeer resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.BGPPeer{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleaning up the BGP resource")
			bgp := &v1alpha1.BGP{}
			Expect(k8sClient.Get(ctx, bgpKey, bgp)).To(Succeed())
			Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())

			By("Cleaning up all Interface resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Interface{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the BGP peer is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.BGPPeers.Has(host)).To(BeFalse(), "Provider should not have BGP peer configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a BGP peer", func() {
			By("Creating a BGPPeer resource")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
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
			By("Creating a Loopback Interface resource on the same device")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "Loopback0",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating a BGPPeer resource with LocalAddress pointing to the Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the controller updates the status conditions successfully")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
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
			By("Creating a BGPPeer resource with LocalAddress pointing to a non-existent Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: "non-existing-interface"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the controller sets Interface not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
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
			By("Creating a Loopback Interface resource on a different device")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:       "Loopback0",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating a BGPPeer resource with LocalAddress pointing to the cross-device Interface")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Address:   host,
					ASNumber:  intstr.FromInt(65000),
					LocalAddress: &v1alpha1.BGPPeerLocalAddress{
						InterfaceRef: v1alpha1.LocalObjectReference{Name: name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the BGP peer rejects the cross-device interface reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
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

		It("Should set Configured=False with BGPNotFoundReason when no BGP resource exists on device", func() {
			By("Creating a separate Device without a BGP resource")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-nobgp-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.3:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			key := client.ObjectKey{Name: device.Name, Namespace: metav1.NamespaceDefault}

			By("Creating a BGPPeer on the device that has no BGP")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      device.Name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Address:   "10.0.0.2",
					ASNumber:  intstr.FromInt(65001),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with BGPNotFoundReason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
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
				g.Expect(testProvider.BGPPeers.Has("10.0.0.2")).To(BeFalse(), "Provider should not have BGP peer configured")
			}).Should(Succeed())

			By("Cleaning up test-specific resources")
			Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
		})

		It("Should set Configured=False with WaitingForDependenciesReason when BGP exists but is not configured", func() {
			By("Creating a separate Device")
			unconfiguredDevice := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-uncfg-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.4:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, unconfiguredDevice)).To(Succeed())
			unconfiguredKey := client.ObjectKey{Name: unconfiguredDevice.Name, Namespace: metav1.NamespaceDefault}

			By("Creating a paused BGP resource with DeviceLabel pre-set (will not be configured)")
			bgp := &v1alpha1.BGP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-bgppeer-uncfg-bgp-",
					Namespace:    metav1.NamespaceDefault,
					Labels: map[string]string{
						v1alpha1.DeviceLabel: unconfiguredDevice.Name,
					},
					Annotations: map[string]string{
						v1alpha1.PausedAnnotation: "",
					},
				},
				Spec: v1alpha1.BGPSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: unconfiguredDevice.Name},
					ASNumber:  intstr.FromInt(65000),
					RouterID:  "10.0.0.11",
				},
			}
			Expect(k8sClient.Create(ctx, bgp)).To(Succeed())

			By("Creating a BGPPeer on the device with the unconfigured BGP")
			bgppeer := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      unconfiguredDevice.Name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.BGPPeerSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: unconfiguredDevice.Name},
					Address:   "10.0.0.3",
					ASNumber:  intstr.FromInt(65002),
				},
			}
			Expect(k8sClient.Create(ctx, bgppeer)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, unconfiguredKey, resource)).To(Succeed())
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

			By("Cleaning up test-specific resources")
			Expect(k8sClient.Delete(ctx, bgppeer)).To(Succeed())
			Expect(k8sClient.Delete(ctx, bgp)).To(Succeed())
			Expect(k8sClient.Delete(ctx, unconfiguredDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
		})
	})
})
