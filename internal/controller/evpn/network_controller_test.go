// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("Network Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			key          client.ObjectKey
			deviceKey    client.ObjectKey
			fabricKey    client.ObjectKey
			vniPoolKey   client.ObjectKey
			interfaceKey client.ObjectKey
		)
		BeforeEach(func() {
			net := &overlayv1alpha1.Network{
				GenerateName: "test-network-",
				Namespace:    metav1.NamespaceDefault,
			}
			Expect(k8sClient.Create(ctx, net)).To(Succeed())
			key = client.ObjectKeyFromObject(net)

			na := &overlayv1alpha1.NetworkAttachment{
				Name:      net.Name,
				Namespace: metav1.NamespaceDefault,
				Spec: overlayv1alpha1.NetworkAttachmentSpec{
					NetworkRef: v1alpha1.LocalObjectReference{Name: net.Name},
					InterfaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"pod": "ap101"},
					},
					Encapsulation: overlayv1alpha1.Encapsulation{
						Type: overlayv1alpha1.EncapsulationTypeVLAN,
						ID:   100,
					},
				},
			}
			Expect(k8sClient.Create(ctx, na)).To(Succeed())

			vniPool := &poolv1alpha1.IndexPool{
				GenerateName: "vni-pool-",
				Namespace:    metav1.NamespaceDefault,
				Spec: poolv1alpha1.IndexPoolSpec{
					Ranges: []v1alpha1.IndexRange{v1alpha1.MustParseIndexRange("10000..10010")},
				},
			}
			Expect(k8sClient.Create(ctx, vniPool)).To(Succeed())
			vniPoolKey = client.ObjectKeyFromObject(vniPool)

			device := &v1alpha1.Device{
				GenerateName: "test-device-",
				Namespace:    metav1.NamespaceDefault,
				Labels:       map[string]string{"fabric-one": "true"},
				Spec: v1alpha1.DeviceSpec{
					Provider: "test-provider",
					Endpoint: v1alpha1.Endpoint{Address: "192.0.2.1:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceKey = client.ObjectKeyFromObject(device)

			fabric := &evpnv1alpha1.Fabric{
				GenerateName: "fabric-one-",
				Namespace:    metav1.NamespaceDefault,
				Spec: evpnv1alpha1.FabricSpec{
					DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"fabric-one": "true"}},
					Loopbacks: evpnv1alpha1.FabricLoopbacksSpec{
						IPAddressPoolRef: v1alpha1.LocalObjectReference{Name: "unused-loopback-pool"},
					},
					Underlay: evpnv1alpha1.FabricUnderlaySpec{
						Protocol:          evpnv1alpha1.UnderlayProtocolOSPF,
						InterfaceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
						Addressing:        evpnv1alpha1.FabricUnderlayAddressingSpec{Unnumbered: true},
					},
					Overlay: evpnv1alpha1.FabricOverlaySpec{
						Protocol: evpnv1alpha1.OverlayProtocolIBGP,
						IBGP: &evpnv1alpha1.FabricIBGPSpec{
							ASNumber: intstr.FromInt(65000),
							RouteReflectors: []evpnv1alpha1.RouteReflectorGroup{{
								Name:                 "unused",
								DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
								ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
							}},
						},
					},
					BUM: evpnv1alpha1.FabricBUMSpec{
						Type: evpnv1alpha1.BUMTypeMulticast,
						PIM: &evpnv1alpha1.FabricPIMSpec{
							AnycastRendezvousPoints: []evpnv1alpha1.AnycastRendezvousPoint{{
								Name:                 "unused",
								MulticastGroups:      []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix("224.0.0.0/4")},
								DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
								ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
							}},
						},
					},
					VTEP: evpnv1alpha1.FabricVTEPSpec{
						DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
					},
					VNIPool: v1alpha1.TypedLocalObjectReference{
						APIVersion: poolv1alpha1.GroupVersion.String(),
						Kind:       "IndexPool",
						Name:       vniPool.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, fabric)).To(Succeed())
			fabricKey = client.ObjectKeyFromObject(fabric)
		})

		AfterEach(func() {
			// Finish Interface cleanup while the attachment's dependencies still exist.
			na := &overlayv1alpha1.NetworkAttachment{Name: key.Name, Namespace: key.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, na))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &overlayv1alpha1.NetworkAttachment{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			// Envtest does not garbage-collect attachment-owned resources.
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EVPNInstance{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.VLAN{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())

			net := &overlayv1alpha1.Network{Name: key.Name, Namespace: key.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, net))).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.Network{}))).To(BeTrue())
			}).Should(Succeed())

			// Stop Fabric reconciliation before removing Claims so they cannot be recreated.
			fabric := &evpnv1alpha1.Fabric{Name: fabricKey.Name, Namespace: fabricKey.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, fabric))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, fabricKey, &evpnv1alpha1.Fabric{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			// Envtest has no garbage collector; Claims must finalize before their pools disappear.
			Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(fabricKey.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: fabricKey.Name})).To(Succeed())
			Eventually(func(g Gomega) {
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(fabricKey.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: fabricKey.Name})).To(Succeed())
				g.Expect(claims.Items).To(BeEmpty())
			}).Should(Succeed())

			// Claim finalization recycles its Index allocation without relying on garbage collection.
			Eventually(func(g Gomega) {
				indices := &poolv1alpha1.IndexList{}
				g.Expect(k8sClient.List(ctx, indices, client.InNamespace(vniPoolKey.Namespace))).To(Succeed())
				g.Expect(indices.Items).NotTo(ContainElement(HaveField("Spec.PoolRef.Name", vniPoolKey.Name)))
			}).Should(Succeed())
			vniPool := &poolv1alpha1.IndexPool{Name: vniPoolKey.Name, Namespace: vniPoolKey.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vniPool))).To(Succeed())

			device := &v1alpha1.Device{Name: deviceKey.Name, Namespace: deviceKey.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		BeforeEach(func() {
			intf := &v1alpha1.Interface{
				GenerateName: "network-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{Mode: v1alpha1.SwitchportModeTrunk},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			interfaceKey = client.ObjectKeyFromObject(intf)
		})
		AfterEach(func() {
			intf := &v1alpha1.Interface{Name: interfaceKey.Name, Namespace: interfaceKey.Namespace}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
		})

		It("successfully allocates and retains a Network VNI", func() {
			By("Resolving the Fabric through the attachment and reporting Network Ready")
			intf := &v1alpha1.Interface{}
			Expect(k8sClient.Get(ctx, interfaceKey, intf)).To(Succeed())
			intf.Labels = map[string]string{"pod": "ap101"}
			Expect(k8sClient.Update(ctx, intf)).To(Succeed())
			net := &overlayv1alpha1.Network{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(net, evpnv1alpha1.FinalizerName)).To(BeTrue())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
					HaveField("ObservedGeneration", net.Generation),
				)))
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.ForwardingContextRef).To(Equal(&v1alpha1.TypedLocalObjectReference{
					APIVersion: evpnv1alpha1.GroupVersion.String(), Kind: "Fabric", Name: fabricKey.Name,
				}))
			}).Should(Succeed())

			By("Allocating one deterministic Fabric-owned Claim from its VNI pool")
			claims := &poolv1alpha1.ClaimList{}
			Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
			Expect(claims.Items).To(HaveLen(1))
			claim := claims.Items[0]
			Expect(claim.Name).To(Equal("vni-" + hash(net)))
			fabric := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
			Expect(metav1.IsControlledBy(&claim, fabric)).To(BeTrue())
			Expect(claim.Labels).To(HaveKeyWithValue(evpnv1alpha1.FabricLabel, fabric.Name))
			Expect(claim.Spec.PoolRef).To(Equal(fabric.Spec.VNIPool))
			Expect(claim.Status.Value).To(Equal("10000"))
			Expect(claim.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", poolv1alpha1.AllocatedCondition), HaveField("Status", metav1.ConditionTrue),
			)))
			Expect(claim.Status.AllocationRef).NotTo(BeNil())
			index := &poolv1alpha1.Index{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: claim.Status.AllocationRef.Name}, index)).To(Succeed())
			Expect(index.Spec.PoolRef.Name).To(Equal(vniPoolKey.Name))

			By("Retaining the allocation after the last attachment disappears")
			na := &overlayv1alpha1.NetworkAttachment{Name: key.Name, Namespace: key.Namespace}
			Expect(k8sClient.Delete(ctx, na)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.NetworkAttachment{}))).To(BeTrue())
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", v1alpha1.ReadyReason),
				)))
				current := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&claim), current)).To(Succeed())
				g.Expect(current.UID).To(Equal(claim.UID))
				g.Expect(current.Status.Value).To(Equal(claim.Status.Value))
				g.Expect(current.DeletionTimestamp.IsZero()).To(BeTrue())
			}).Should(Succeed())
		})

		It("waits for attachment resolution and recovers when an Interface matches", func() {
			By("Waiting without allocating while no Interface matches the attachment")
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", v1alpha1.WaitingForDependenciesReason),
				)))
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(BeEmpty())
			}).Should(Succeed())
			By("Resolving through the attachment status watch after its selector matches")
			intf := &v1alpha1.Interface{}
			Expect(k8sClient.Get(ctx, interfaceKey, intf)).To(Succeed())
			intf.Labels = map[string]string{"pod": "ap101"}
			Expect(k8sClient.Update(ctx, intf)).To(Succeed())
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())
		})

		It("preserves the allocation when the attachment resolves to another Fabric", func() {
			intf := &v1alpha1.Interface{}
			Expect(k8sClient.Get(ctx, interfaceKey, intf)).To(Succeed())
			intf.Labels = map[string]string{"pod": "ap101"}
			Expect(k8sClient.Update(ctx, intf)).To(Succeed())
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())
			claims := &poolv1alpha1.ClaimList{}
			Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
			Expect(claims.Items).To(HaveLen(1))
			claim := claims.Items[0]

			By("Creating another Fabric and moving the selected Device into it")
			fabric := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
			other := &evpnv1alpha1.Fabric{
				GenerateName: "other-fabric-", Namespace: key.Namespace, Spec: fabric.Spec,
			}
			other.Spec.DeviceSelector = metav1.LabelSelector{MatchLabels: map[string]string{"other-fabric": key.Name}}
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, other))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(other), &evpnv1alpha1.Fabric{}))).To(BeTrue())
				}).Should(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: other.Name})).To(Succeed())
			})
			device := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
			device.Labels = map[string]string{"other-fabric": key.Name}
			Expect(k8sClient.Update(ctx, device)).To(Succeed())
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.StretchedNetworkNotSupportedReason),
				)))
				current := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&claim), current)).To(Succeed())
				g.Expect(current.UID).To(Equal(claim.UID))
				g.Expect(current.OwnerReferences).To(Equal(claim.OwnerReferences))
				g.Expect(current.Spec.PoolRef).To(Equal(claim.Spec.PoolRef))
				g.Expect(current.Status.Value).To(Equal(claim.Status.Value))
			}).Should(Succeed())
		})
	})
})
