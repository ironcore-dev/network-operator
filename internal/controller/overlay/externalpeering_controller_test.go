// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("ExternalPeering Controller", func() {
	Context("When reconciling the success case", func() {
		var (
			loopbackPool *poolv1alpha1.IPAddressPool
			device       *corev1alpha1.Device
			rd           *overlayv1alpha1.RoutingDomain
			vrf          *corev1alpha1.VRF
			defaultBGP   *corev1alpha1.BGP
		)

		BeforeEach(func() {
			By("Creating an IPAddressPool for loopback allocation")
			loopbackPool = &poolv1alpha1.IPAddressPool{
				GenerateName: "loopback-pool-",
				Namespace:    metav1.NamespaceDefault,
				Spec: poolv1alpha1.IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				},
			}
			Expect(k8sClient.Create(ctx, loopbackPool)).To(Succeed())

			By("Creating a device matching the peering selector")
			device = &corev1alpha1.Device{
				GenerateName: "leaf-",
				Namespace:    metav1.NamespaceDefault,
				Labels:       map[string]string{"role": "leaf"},
				Spec:         corev1alpha1.DeviceSpec{Provider: "test-provider", Endpoint: corev1alpha1.Endpoint{Address: "192.168.1.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Creating a RoutingDomain")
			rd = &overlayv1alpha1.RoutingDomain{
				GenerateName: "rd-",
				Namespace:    metav1.NamespaceDefault,
				Spec:         overlayv1alpha1.RoutingDomainSpec{Name: "CUSTOMER-A"},
			}
			Expect(k8sClient.Create(ctx, rd)).To(Succeed())

			By("Creating a VRF on the device owned by the RoutingDomain")
			vrf = &corev1alpha1.VRF{
				GenerateName: "vrf-",
				Namespace:    metav1.NamespaceDefault,
				Spec: corev1alpha1.VRFSpec{
					DeviceRef: corev1alpha1.LocalObjectReference{Name: device.Name},
					Name:      "CUSTOMER-A",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			Expect(controllerutil.SetControllerReference(rd, vrf, k8sManager.GetScheme())).To(Succeed())
			Expect(k8sClient.Update(ctx, vrf)).To(Succeed())

			By("Creating a default BGP instance on the device")
			defaultBGP = &corev1alpha1.BGP{
				GenerateName: "bgp-default-",
				Namespace:    metav1.NamespaceDefault,
				Spec: corev1alpha1.BGPSpec{
					DeviceRef:  corev1alpha1.LocalObjectReference{Name: device.Name},
					ASNumber:   intstr.FromInt(65000),
					AdminState: corev1alpha1.AdminStateUp,
					RouterID:   "10.0.0.1",
				},
			}
			Expect(k8sClient.Create(ctx, defaultBGP)).To(Succeed())
		})

		AfterEach(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, defaultBGP))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vrf))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, rd))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, loopbackPool))).To(Succeed())
		})

		It("Should create Claim, Interface, BGP, and BGPPeer for the matching device", func() {
			By("Creating the ExternalPeering resource")
			peering := &overlayv1alpha1.ExternalPeering{
				GenerateName: "ext-peer-",
				Namespace:    metav1.NamespaceDefault,
				Spec: overlayv1alpha1.ExternalPeeringSpec{
					DeviceSelector:   metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
					RoutingDomainRef: corev1alpha1.LocalObjectReference{Name: rd.Name},
					Peering: overlayv1alpha1.Peering{
						SourceAddress: overlayv1alpha1.SourceAddress{
							Type:             overlayv1alpha1.SourceAddressTypeLoopback,
							Name:             "lo10",
							IPAddressPoolRef: corev1alpha1.LocalObjectReference{Name: loopbackPool.Name},
						},
						LocalASNumber:  intstr.FromInt(65100),
						RemoteASNumber: intstr.FromInt(65200),
						ListenRange:    corev1alpha1.MustParsePrefix("10.69.69.69/32"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, peering)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, peering))).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				Eventually(func(g Gomega) {
					list := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, list, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())
			})

			claimName := fmt.Sprintf("%s-%s-lo10", peering.Name, device.Name)

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				p := &overlayv1alpha1.ExternalPeering{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(peering), p)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(p, overlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying a loopback Claim is created for the device")
			Eventually(func(g Gomega) {
				claim := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: claimName, Namespace: metav1.NamespaceDefault}, claim)).To(Succeed())
				g.Expect(claim.Spec.PoolRef.Name).To(Equal(loopbackPool.Name))
				g.Expect(claim.Spec.PoolRef.Kind).To(Equal("IPAddressPool"))
				g.Expect(claim.Labels[overlayv1alpha1.ExternalPeeringLabel]).To(Equal(peering.Name))
				g.Expect(claim.OwnerReferences).To(ContainElement(SatisfyAll(
					HaveField("Kind", "ExternalPeering"),
					HaveField("Name", peering.Name),
					HaveField("Controller", HaveValue(BeTrue())),
				)))
			}).Should(Succeed())

			intfName := fmt.Sprintf("%s-%s-lo10", peering.Name, device.Name)

			By("Verifying a loopback Interface is created once the Claim is allocated")
			Eventually(func(g Gomega) {
				intf := &corev1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: intfName, Namespace: metav1.NamespaceDefault}, intf)).To(Succeed())
				g.Expect(intf.Spec.Type).To(Equal(corev1alpha1.InterfaceTypeLoopback))
				g.Expect(intf.Spec.DeviceRef.Name).To(Equal(device.Name))
				g.Expect(intf.Spec.Name).To(Equal("lo10"))
				g.Expect(intf.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
				g.Expect(intf.Spec.Description).To(Equal("External Peering"))
				g.Expect(intf.Spec.IPv4).NotTo(BeNil())
				g.Expect(intf.Spec.IPv4.Addresses).To(HaveLen(1))
				g.Expect(intf.Labels[overlayv1alpha1.ExternalPeeringLabel]).To(Equal(peering.Name))
			}).Should(Succeed())

			bgpName := fmt.Sprintf("%s-%s", peering.Name, device.Name)

			By("Verifying a BGP instance is created in the VRF")
			Eventually(func(g Gomega) {
				bgp := &corev1alpha1.BGP{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bgpName, Namespace: metav1.NamespaceDefault}, bgp)).To(Succeed())
				g.Expect(bgp.Spec.DeviceRef.Name).To(Equal(device.Name))
				g.Expect(bgp.Spec.VrfRef).NotTo(BeNil())
				g.Expect(bgp.Spec.VrfRef.Name).To(Equal(vrf.Name))
				g.Expect(bgp.Spec.ASNumber).To(Equal(intstr.FromInt(65100)))
				g.Expect(bgp.Spec.RouterID).To(Equal(defaultBGP.Spec.RouterID))
				g.Expect(bgp.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
				g.Expect(bgp.Labels[overlayv1alpha1.ExternalPeeringLabel]).To(Equal(peering.Name))
			}).Should(Succeed())

			By("Verifying a BGPPeer is created with the correct address and AS numbers")
			Eventually(func(g Gomega) {
				peer := &corev1alpha1.BGPPeer{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bgpName, Namespace: metav1.NamespaceDefault}, peer)).To(Succeed())
				g.Expect(peer.Spec.DeviceRef.Name).To(Equal(device.Name))
				g.Expect(peer.Spec.BgpRef.Name).To(Equal(bgpName))
				g.Expect(peer.Spec.ASNumber).To(Equal(intstr.FromInt(65200)))
				g.Expect(peer.Spec.Address).To(Equal("10.69.69.69"))
				g.Expect(peer.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
				g.Expect(peer.Spec.LocalAddress).NotTo(BeNil())
				g.Expect(peer.Spec.LocalAddress.InterfaceRef.Name).To(Equal(intfName))
				g.Expect(peer.Labels[overlayv1alpha1.ExternalPeeringLabel]).To(Equal(peering.Name))
			}).Should(Succeed())
		})
	})
})
