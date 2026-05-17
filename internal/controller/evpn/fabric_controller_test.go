// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

// Minimal 2-spine / 2-leaf EVPN/VXLAN topology used by the tests below.
// All four devices share the "topology.kubernetes.io/zone: test-zone" label;
// the "role" label drives per-role loopback allocation.
//
//	(RR, RP)          (RR, RP)
//	spine-1           spine-2
//	|   \_____________/   |
//	|   /             \   |
//	leaf-1            leaf-2
//	(VTEP)            (VTEP)
//
// Loopback allocation:
//
//	lo0  (Router-ID)    — all 4 devices
//	lo1  (primary VTEP) — leaf-1, leaf-2
//	lo2  (anycast VTEP) — leaf-1, leaf-2
//	lo100 (anycast RP)  — 1 Claim shared across spine-1, spine-2
var _ = Describe("Fabric Controller", func() {
	Context("When reconciling with unnumbered underlay addressing", func() {
		var (
			loopbackPool *poolv1alpha1.IPAddressPool
			spine1       *corev1alpha1.Device
			spine2       *corev1alpha1.Device
			leaf1        *corev1alpha1.Device
			leaf2        *corev1alpha1.Device
			spineIntf    *corev1alpha1.Interface
			leafIntf     *corev1alpha1.Interface
		)

		BeforeEach(func() {
			By("Creating an IPAddressPool for loopback allocation")
			loopbackPool = &poolv1alpha1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "loopback-pool-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: poolv1alpha1.IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				},
			}
			Expect(k8sClient.Create(ctx, loopbackPool)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, loopbackPool))).To(Succeed())
			})

			By("Creating spine-1 (route reflector, rendezvous point)")
			spine1 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "spine1-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "spine"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.0.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, spine1)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, spine1))).To(Succeed())
			})

			By("Creating spine-2 (route reflector, rendezvous point)")
			spine2 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "spine2-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "spine"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.0.2:9339"}},
			}
			Expect(k8sClient.Create(ctx, spine2)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, spine2))).To(Succeed())
			})

			By("Creating leaf-1 (VTEP)")
			leaf1 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "leaf1-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "leaf"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.1.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, leaf1)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, leaf1))).To(Succeed())
			})

			By("Creating leaf-2 (VTEP)")
			leaf2 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "leaf2-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "leaf"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.1.2:9339"}},
			}
			Expect(k8sClient.Create(ctx, leaf2)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, leaf2))).To(Succeed())
			})

			By("Creating a fabric-facing Interface on spine-1")
			spineIntf = &corev1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: spine1.Name + "-eth0-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"role": "fabric"},
				},
				Spec: corev1alpha1.InterfaceSpec{
					DeviceRef:  corev1alpha1.LocalObjectReference{Name: spine1.Name},
					Name:       "eth0",
					Type:       corev1alpha1.InterfaceTypePhysical,
					AdminState: corev1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, spineIntf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, spineIntf))).To(Succeed())
			})

			By("Creating a fabric-facing Interface on leaf-1")
			leafIntf = &corev1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: leaf1.Name + "-eth0-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"role": "fabric"},
				},
				Spec: corev1alpha1.InterfaceSpec{
					DeviceRef:  corev1alpha1.LocalObjectReference{Name: leaf1.Name},
					Name:       "eth0",
					Type:       corev1alpha1.InterfaceTypePhysical,
					AdminState: corev1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, leafIntf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, leafIntf))).To(Succeed())
			})
		})

		It("Should create lo0 Claims for all fabric devices, lo1/lo2 Claims for VTEP devices, and one lo100 Claim per RP group", func() {
			By("Creating the Fabric resource with unnumbered addressing")
			fabric := &evpnv1alpha1.Fabric{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "fabric-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: evpnv1alpha1.FabricSpec{
					DeviceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"topology.kubernetes.io/zone": "test-zone"},
					},
					Loopbacks: evpnv1alpha1.FabricLoopbacksSpec{
						IPAddressPoolRef: corev1alpha1.LocalObjectReference{Name: loopbackPool.Name},
					},
					Underlay: evpnv1alpha1.FabricUnderlaySpec{
						Protocol:          evpnv1alpha1.UnderlayProtocolOSPF,
						InterfaceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "fabric"}},
						Addressing:        evpnv1alpha1.FabricUnderlayAddressingSpec{Unnumbered: true},
					},
					Overlay: evpnv1alpha1.FabricOverlaySpec{
						Protocol: evpnv1alpha1.OverlayProtocolIBGP,
						IBGP: &evpnv1alpha1.FabricIBGPSpec{
							ASNumber: intstr.FromInt(65000),
							RouteReflectors: []evpnv1alpha1.RouteReflectorGroup{
								{
									Name:                 "spines",
									DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"role": "spine"}},
									ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
								},
							},
						},
					},
					BUM: evpnv1alpha1.FabricBUMSpec{
						Type: evpnv1alpha1.BUMTypeMulticast,
						PIM: &evpnv1alpha1.FabricPIMSpec{
							AnycastRendezvousPoints: []evpnv1alpha1.AnycastRendezvousPoint{
								{
									Name:                 "spine-rp",
									MulticastGroups:      []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("224.0.0.0/4")},
									DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"role": "spine"}},
									ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
								},
							},
						},
					},
					VTEP: evpnv1alpha1.FabricVTEPSpec{
						DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, fabric)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, fabric))).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				Eventually(func(g Gomega) {
					list := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, list, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())
			})

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				f := &evpnv1alpha1.Fabric{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fabric), f)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(f, evpnv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying lo0 Claims are created for all 4 fabric devices")
			for _, d := range []*corev1alpha1.Device{spine1, spine2, leaf1, leaf2} {
				Eventually(func(g Gomega) {
					claim := &poolv1alpha1.Claim{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-" + d.Name + "-lo0", Namespace: metav1.NamespaceDefault}, claim)).To(Succeed())
					g.Expect(claim.Spec.PoolRef.Name).To(Equal(loopbackPool.Name))
					g.Expect(claim.Spec.PoolRef.Kind).To(Equal("IPAddressPool"))
					g.Expect(claim.OwnerReferences).To(ContainElement(
						SatisfyAll(
							HaveField("Kind", "Fabric"),
							HaveField("Name", fabric.Name),
							HaveField("Controller", HaveValue(BeTrue())),
						),
					))
				}).Should(Succeed())
			}

			By("Verifying lo1 and lo2 Claims are created only for leaf (VTEP) devices")
			for _, d := range []*corev1alpha1.Device{leaf1, leaf2} {
				for _, id := range []string{"lo1", "lo2"} {
					Eventually(func(g Gomega) {
						claim := &poolv1alpha1.Claim{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-" + d.Name + "-" + id, Namespace: metav1.NamespaceDefault}, claim)).To(Succeed())
					}).Should(Succeed())
				}
			}

			By("Verifying a single lo100 Claim is created for the spine-rp RP group")
			Eventually(func(g Gomega) {
				claim := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-spine-rp-lo100", Namespace: metav1.NamespaceDefault}, claim)).To(Succeed())
			}).Should(Succeed())

			By("Verifying lo0 Interfaces are created for all 4 fabric devices once Claims are allocated")
			for _, d := range []*corev1alpha1.Device{spine1, spine2, leaf1, leaf2} {
				Eventually(func(g Gomega) {
					intf := &corev1alpha1.Interface{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-" + d.Name + "-lo0", Namespace: metav1.NamespaceDefault}, intf)).To(Succeed())
					g.Expect(intf.Spec.Type).To(Equal(corev1alpha1.InterfaceTypeLoopback))
					g.Expect(intf.Spec.DeviceRef.Name).To(Equal(d.Name))
					g.Expect(intf.Spec.Name).To(Equal("lo0"))
					g.Expect(intf.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
					g.Expect(intf.Spec.Description).To(Equal("Router-ID, BGP Source"))
					g.Expect(intf.Spec.IPv4).NotTo(BeNil())
					g.Expect(intf.Spec.IPv4.Addresses).To(HaveLen(1))
				}).Should(Succeed())
			}

			By("Verifying lo1 and lo2 Interfaces are created for leaf (VTEP) devices")
			for _, d := range []*corev1alpha1.Device{leaf1, leaf2} {
				for loIdx, id := range []string{"lo1", "lo2"} {
					descriptions := []string{"Primary VTEP", "VTEP Anycast"}
					Eventually(func(g Gomega) {
						intf := &corev1alpha1.Interface{}
						g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-" + d.Name + "-" + id, Namespace: metav1.NamespaceDefault}, intf)).To(Succeed())
						g.Expect(intf.Spec.Type).To(Equal(corev1alpha1.InterfaceTypeLoopback))
						g.Expect(intf.Spec.DeviceRef.Name).To(Equal(d.Name))
						g.Expect(intf.Spec.Name).To(Equal(id))
						g.Expect(intf.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
						g.Expect(intf.Spec.Description).To(Equal(descriptions[loIdx]))
						g.Expect(intf.Spec.IPv4).NotTo(BeNil())
						g.Expect(intf.Spec.IPv4.Addresses).To(HaveLen(1))
					}).Should(Succeed())
				}
			}

			By("Verifying the lo100 Interface is created on each spine (shared RP address)")
			for _, d := range []*corev1alpha1.Device{spine1, spine2} {
				Eventually(func(g Gomega) {
					intf := &corev1alpha1.Interface{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: fabric.Name + "-" + d.Name + "-lo100", Namespace: metav1.NamespaceDefault}, intf)).To(Succeed())
					g.Expect(intf.Spec.Type).To(Equal(corev1alpha1.InterfaceTypeLoopback))
					g.Expect(intf.Spec.DeviceRef.Name).To(Equal(d.Name))
					g.Expect(intf.Spec.Name).To(Equal("lo100"))
					g.Expect(intf.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
					g.Expect(intf.Spec.Description).To(Equal("Rendezvous Point"))
					g.Expect(intf.Spec.IPv4).NotTo(BeNil())
					g.Expect(intf.Spec.IPv4.Addresses).To(HaveLen(1))
				}).Should(Succeed())
			}

			By("Verifying underlay interfaces are patched with MTU 9216 and unnumbered IPv4 pointing to lo0")
			for _, intf := range []*corev1alpha1.Interface{spineIntf, leafIntf} {
				Eventually(func(g Gomega) {
					i := &corev1alpha1.Interface{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), i)).To(Succeed())
					g.Expect(i.Spec.MTU).To(Equal(int32(9216)))
					g.Expect(i.Spec.IPv4).NotTo(BeNil())
					g.Expect(i.Spec.IPv4.Unnumbered).NotTo(BeNil())
					g.Expect(i.Spec.IPv4.Unnumbered.InterfaceRef.Name).To(Equal(fabric.Name + "-" + i.Spec.DeviceRef.Name + "-lo0"))
				}).Should(Succeed())
			}

			By("Verifying the Fabric Ready condition is True once all phases are complete")
			Eventually(func(g Gomega) {
				f := &evpnv1alpha1.Fabric{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fabric), f)).To(Succeed())
				g.Expect(f.Status.Conditions).To(ContainElement(
					SatisfyAll(
						HaveField("Type", corev1alpha1.ReadyCondition),
						HaveField("Status", metav1.ConditionTrue),
					),
				))
			}).Should(Succeed())
		})
	})

	Context("When reconciling with numbered underlay addressing", func() {
		var (
			loopbackPool *poolv1alpha1.IPAddressPool
			prefixPool   *poolv1alpha1.IPPrefixPool
			spine1       *corev1alpha1.Device
			leaf1        *corev1alpha1.Device
			spineIntf    *corev1alpha1.Interface
			leafIntf     *corev1alpha1.Interface
		)

		BeforeEach(func() {
			By("Creating an IPAddressPool for loopback allocation")
			loopbackPool = &poolv1alpha1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "loopback-pool-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: poolv1alpha1.IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				},
			}
			Expect(k8sClient.Create(ctx, loopbackPool)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, loopbackPool))).To(Succeed())
			})

			By("Creating an IPPrefixPool for P2P link allocation")
			prefixPool = &poolv1alpha1.IPPrefixPool{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "p2p-pool-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: poolv1alpha1.IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.1.0.0/16")},
					AllocationPrefixLength: 31,
				},
			}
			Expect(k8sClient.Create(ctx, prefixPool)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, prefixPool))).To(Succeed())
			})

			By("Creating spine-1")
			spine1 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "spine1-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "spine"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.0.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, spine1)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, spine1))).To(Succeed())
			})

			By("Creating leaf-1")
			leaf1 = &corev1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "leaf1-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"topology.kubernetes.io/zone": "test-zone", "role": "leaf"},
				},
				Spec: corev1alpha1.DeviceSpec{Endpoint: corev1alpha1.Endpoint{Address: "192.168.1.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, leaf1)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, leaf1))).To(Succeed())
			})

			By("Creating a fabric-facing Interface on spine-1 with neighbor label pointing to leaf-1's interface")
			// Create both interfaces first, then set the neighbor labels so each points to the other.
			spineIntf = &corev1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: spine1.Name + "-eth0-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"role": "fabric"},
				},
				Spec: corev1alpha1.InterfaceSpec{
					DeviceRef:  corev1alpha1.LocalObjectReference{Name: spine1.Name},
					Name:       "eth0",
					Type:       corev1alpha1.InterfaceTypePhysical,
					AdminState: corev1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, spineIntf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, spineIntf))).To(Succeed())
			})

			leafIntf = &corev1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: leaf1.Name + "-eth0-",
					Namespace:    metav1.NamespaceDefault,
					Labels:       map[string]string{"role": "fabric"},
				},
				Spec: corev1alpha1.InterfaceSpec{
					DeviceRef:  corev1alpha1.LocalObjectReference{Name: leaf1.Name},
					Name:       "eth0",
					Type:       corev1alpha1.InterfaceTypePhysical,
					AdminState: corev1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, leafIntf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, leafIntf))).To(Succeed())
			})

			By("Setting neighbor labels to pair the interfaces as a link")
			spineOrig := spineIntf.DeepCopy()
			spineIntf.Labels[corev1alpha1.PhysicalInterfaceNeighborLabel] = leafIntf.Name
			Expect(k8sClient.Patch(ctx, spineIntf, client.MergeFrom(spineOrig))).To(Succeed())

			leafOrig := leafIntf.DeepCopy()
			leafIntf.Labels[corev1alpha1.PhysicalInterfaceNeighborLabel] = spineIntf.Name
			Expect(k8sClient.Patch(ctx, leafIntf, client.MergeFrom(leafOrig))).To(Succeed())
		})

		It("Should allocate one /31 prefix Claim per link and assign host addresses to both ends", func() {
			By("Creating the Fabric resource with numbered addressing")
			fabric := &evpnv1alpha1.Fabric{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "fabric-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: evpnv1alpha1.FabricSpec{
					DeviceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"topology.kubernetes.io/zone": "test-zone"},
					},
					Loopbacks: evpnv1alpha1.FabricLoopbacksSpec{
						IPAddressPoolRef: corev1alpha1.LocalObjectReference{Name: loopbackPool.Name},
					},
					Underlay: evpnv1alpha1.FabricUnderlaySpec{
						Protocol:          evpnv1alpha1.UnderlayProtocolOSPF,
						InterfaceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "fabric"}},
						Addressing: evpnv1alpha1.FabricUnderlayAddressingSpec{
							IPPrefixPoolRef: &corev1alpha1.LocalObjectReference{Name: prefixPool.Name},
						},
					},
					Overlay: evpnv1alpha1.FabricOverlaySpec{
						Protocol: evpnv1alpha1.OverlayProtocolIBGP,
						IBGP: &evpnv1alpha1.FabricIBGPSpec{
							ASNumber: intstr.FromInt(65000),
							RouteReflectors: []evpnv1alpha1.RouteReflectorGroup{
								{
									Name:                 "spines",
									DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"role": "spine"}},
									ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
								},
							},
						},
					},
					BUM: evpnv1alpha1.FabricBUMSpec{
						Type: evpnv1alpha1.BUMTypeMulticast,
						PIM: &evpnv1alpha1.FabricPIMSpec{
							AnycastRendezvousPoints: []evpnv1alpha1.AnycastRendezvousPoint{
								{
									Name:                 "spine-rp",
									MulticastGroups:      []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("224.0.0.0/4")},
									DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"role": "spine"}},
									ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
								},
							},
						},
					},
					VTEP: evpnv1alpha1.FabricVTEPSpec{
						DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "leaf"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, fabric)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, fabric))).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				Eventually(func(g Gomega) {
					list := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, list, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())
			})

			// Stable claim name: alphabetically sorted interface names.
			claimName := fabric.Name + "-" + min(spineIntf.Name, leafIntf.Name) + "-" + max(spineIntf.Name, leafIntf.Name) + "-p2p"

			By("Verifying a single p2p Claim is created for the link")
			Eventually(func(g Gomega) {
				claim := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: claimName, Namespace: metav1.NamespaceDefault}, claim)).To(Succeed())
				g.Expect(claim.Spec.PoolRef.Kind).To(Equal("IPPrefixPool"))
				g.Expect(claim.Spec.PoolRef.Name).To(Equal(prefixPool.Name))
			}).Should(Succeed())

			By("Verifying the spine-side interface is patched with MTU 9216 and a /31 address")
			Eventually(func(g Gomega) {
				i := &corev1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(spineIntf), i)).To(Succeed())
				g.Expect(i.Spec.MTU).To(Equal(int32(9216)))
				g.Expect(i.Spec.IPv4).NotTo(BeNil())
				g.Expect(i.Spec.IPv4.Addresses).To(HaveLen(1))
				g.Expect(i.Spec.IPv4.Addresses[0].Bits()).To(Equal(31))
			}).Should(Succeed())

			By("Verifying the leaf-side interface is patched with MTU 9216 and the peer /31 address")
			Eventually(func(g Gomega) {
				i := &corev1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(leafIntf), i)).To(Succeed())
				g.Expect(i.Spec.MTU).To(Equal(int32(9216)))
				g.Expect(i.Spec.IPv4).NotTo(BeNil())
				g.Expect(i.Spec.IPv4.Addresses).To(HaveLen(1))
				g.Expect(i.Spec.IPv4.Addresses[0].Bits()).To(Equal(31))
			}).Should(Succeed())

			By("Verifying the two addresses are adjacent and from the same /31")
			Eventually(func(g Gomega) {
				si := &corev1alpha1.Interface{}
				li := &corev1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(spineIntf), si)).To(Succeed())
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(leafIntf), li)).To(Succeed())
				g.Expect(si.Spec.IPv4.Addresses).To(HaveLen(1))
				g.Expect(li.Spec.IPv4.Addresses).To(HaveLen(1))
				spineAddr := si.Spec.IPv4.Addresses[0].Addr()
				leafAddr := li.Spec.IPv4.Addresses[0].Addr()
				// The two addresses must differ and share the same masked prefix.
				g.Expect(spineAddr).NotTo(Equal(leafAddr))
				g.Expect(si.Spec.IPv4.Addresses[0].Masked()).To(Equal(li.Spec.IPv4.Addresses[0].Masked()))
			}).Should(Succeed())
		})
	})
})
