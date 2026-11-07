// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"cmp"
	"slices"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("NetworkAttachment Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			key        client.ObjectKey
			deviceKey  client.ObjectKey
			fabricKey  client.ObjectKey
			vniPoolKey client.ObjectKey
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

		It("successfully realizes a NetworkAttachment", func() {
			By("Reporting no matching Interfaces before any Interface is created")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NoMatchingInterfacesReason),
				)))
			}).Should(Succeed())

			By("Creating an unselected Physical member for the Aggregate Interface")
			member := &v1alpha1.Interface{
				GenerateName: "aggregate-member-",
				Namespace:    key.Namespace,
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/2",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, member))).To(Succeed())
			})

			By("Creating selected Physical and Aggregate trunks on the same Device")
			interfaces := []*v1alpha1.Interface{
				{
					GenerateName: "attachment-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap101"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Ethernet1/1",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypePhysical,
						Switchport: &v1alpha1.Switchport{Mode: v1alpha1.SwitchportModeTrunk},
					},
				},
				{
					GenerateName: "attachment-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap101"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Port-Channel1",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypeAggregate,
						Aggregation: &v1alpha1.Aggregation{
							MemberInterfaceRefs: []v1alpha1.LocalObjectReference{{Name: member.Name}},
							ControlProtocol:     v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
						},
						Switchport: &v1alpha1.Switchport{
							Mode:             v1alpha1.SwitchportModeTrunk,
							AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
							AllowedVlans:     []v1alpha1.IndexRange{{Start: 10, End: 20}, {Start: 200, End: 200}},
						},
					},
				},
			}
			for _, intf := range interfaces {
				Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			}
			DeferCleanup(func() {
				for _, intf := range interfaces {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
				}
			})

			fabric := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())

			na := &overlayv1alpha1.NetworkAttachment{}
			Expect(k8sClient.Get(ctx, key, na)).To(Succeed())

			By("Allocating one VNI Claim owned by the Fabric")
			var allocatedVNI string
			Eventually(func(g Gomega) {
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(HaveLen(1))
				claim := &claims.Items[0]
				g.Expect(claim.Spec.PoolRef).To(Equal(fabric.Spec.VNIPool))
				g.Expect(claim.Labels).To(HaveKeyWithValue(evpnv1alpha1.FabricLabel, fabric.Name))
				g.Expect(metav1.IsControlledBy(claim, fabric)).To(BeTrue())
				g.Expect(claim.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", poolv1alpha1.AllocatedCondition),
					HaveField("Status", metav1.ConditionTrue),
				)))
				g.Expect(claim.Status.Value).To(Equal("10000"))
				g.Expect(claim.Status.AllocationRef).NotTo(BeNil())
				g.Expect(claim.Status.AllocationRef.Kind).To(Equal("Index"))
				allocatedVNI = claim.Status.Value
			}).Should(Succeed())

			By("Reporting the Network allocation as Ready")
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())

			By("Creating one VLAN and bridged EVPNInstance for the selected Device")
			Eventually(func(g Gomega) {
				vlans := &v1alpha1.VLANList{}
				g.Expect(k8sManager.GetClient().List(ctx, vlans, client.InNamespace(key.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(na.UID)})).To(Succeed())
				g.Expect(vlans.Items).To(HaveLen(1))
				vlan := &vlans.Items[0]
				g.Expect(vlan.Spec.DeviceRef.Name).To(Equal(deviceKey.Name))
				g.Expect(vlan.Spec.ID).To(Equal(int16(100)))
				g.Expect(vlan.Spec.AdminState).To(Equal(v1alpha1.AdminStateUp))
				g.Expect(metav1.IsControlledBy(vlan, na)).To(BeTrue())
				g.Expect(vlan.Labels).To(SatisfyAll(
					HaveKeyWithValue(evpnv1alpha1.NetworkLabel, key.Name),
					HaveKeyWithValue(evpnv1alpha1.NetworkAttachmentLabel, na.Name),
					HaveKeyWithValue(evpnv1alpha1.FabricLabel, fabric.Name),
				))

				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sManager.GetClient().List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(na.UID)})).To(Succeed())
				g.Expect(evis.Items).To(HaveLen(1))
				evi := &evis.Items[0]
				g.Expect(evi.Spec.DeviceRef.Name).To(Equal(deviceKey.Name))
				g.Expect(evi.Spec.VNI).To(Equal(int32(10000)))
				g.Expect(strconv.FormatInt(int64(evi.Spec.VNI), 10)).To(Equal(allocatedVNI))
				g.Expect(evi.Spec.Type).To(Equal(v1alpha1.EVPNInstanceTypeBridged))
				g.Expect(evi.Spec.RouteDistinguisher).To(Equal("Auto"))
				g.Expect(evi.Spec.RouteTargets).To(Equal([]v1alpha1.EVPNRouteTarget{{Value: "65000:268445456", Action: v1alpha1.RouteTargetActionBoth}}))
				g.Expect(evi.Spec.VLANRef).To(Equal(&v1alpha1.LocalObjectReference{Name: vlan.Name}))
				g.Expect(metav1.IsControlledBy(evi, na)).To(BeTrue())
				g.Expect(evi.Labels).To(SatisfyAll(
					HaveKeyWithValue(evpnv1alpha1.NetworkLabel, key.Name),
					HaveKeyWithValue(evpnv1alpha1.NetworkAttachmentLabel, na.Name),
					HaveKeyWithValue(evpnv1alpha1.FabricLabel, fabric.Name),
				))
			}).Should(Succeed())

			By("Adding VLAN membership while preserving unrelated allowed VLANs")
			Eventually(func(g Gomega) {
				first := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[0]), first)).To(Succeed())
				g.Expect(first.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
				second := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[1]), second)).To(Succeed())
				g.Expect(second.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 10, End: 20}, {Start: 100, End: 100}, {Start: 200, End: 200}}))
			}).Should(Succeed())

			expectedStatus := []overlayv1alpha1.NetworkAttachmentInterfaceStatus{
				{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceKey.Name}, InterfaceRef: v1alpha1.LocalObjectReference{Name: interfaces[0].Name}},
				{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceKey.Name}, InterfaceRef: v1alpha1.LocalObjectReference{Name: interfaces[1].Name}},
			}
			slices.SortFunc(expectedStatus, func(a, b overlayv1alpha1.NetworkAttachmentInterfaceStatus) int {
				return cmp.Compare(a.InterfaceRef.Name, b.InterfaceRef.Name)
			})
			By("Recording ownership and forwarding context and reporting Ready")
			Eventually(func(g Gomega) {
				current := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(current, evpnv1alpha1.FinalizerName)).To(BeTrue())
				g.Expect(current.Status.Interfaces).To(Equal(expectedStatus))
				g.Expect(current.Status.ForwardingContextRef).To(Equal(&v1alpha1.TypedLocalObjectReference{
					APIVersion: evpnv1alpha1.GroupVersion.String(),
					Kind:       "Fabric",
					Name:       fabric.Name,
				}))
				g.Expect(current.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
					HaveField("ObservedGeneration", current.Generation),
				)))
			}).Should(Succeed())
		})

		It("shares the Network allocation between attachments", func() {
			By("Creating an unresolved attachment that must not block allocation")
			unresolved := &overlayv1alpha1.NetworkAttachment{
				GenerateName: "unresolved-attachment-",
				Namespace:    key.Namespace,
				Spec: overlayv1alpha1.NetworkAttachmentSpec{
					NetworkRef:        v1alpha1.LocalObjectReference{Name: key.Name},
					InterfaceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
					Encapsulation:     overlayv1alpha1.Encapsulation{Type: overlayv1alpha1.EncapsulationTypeVLAN, ID: 102},
				},
			}
			Expect(k8sClient.Create(ctx, unresolved)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, unresolved))).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(unresolved), &overlayv1alpha1.NetworkAttachment{})
					g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			})
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(unresolved), na)).To(Succeed())
				g.Expect(na.Status.ForwardingContextRef).To(BeNil())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NoMatchingInterfacesReason),
				)))
			}).Should(Succeed())

			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{Mode: v1alpha1.SwitchportModeTrunk},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Waiting for the first attachment to bind the Network")
			Eventually(func(g Gomega) {
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())
			claims := &poolv1alpha1.ClaimList{}
			Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
			Expect(claims.Items).To(HaveLen(1))
			claim := claims.Items[0]

			By("Adding another attachment on a second Device in the same Fabric")
			device := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
			otherDevice := &v1alpha1.Device{
				GenerateName: "shared-device-",
				Namespace:    key.Namespace,
				Labels:       device.Labels,
				Spec:         device.Spec,
			}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherDevice))).To(Succeed())
			})
			otherInterface := &v1alpha1.Interface{
				GenerateName: "shared-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap102"},
				Spec:         intf.Spec,
			}
			otherInterface.Spec.DeviceRef.Name = otherDevice.Name
			Expect(k8sClient.Create(ctx, otherInterface)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherInterface))).To(Succeed())
			})
			other := &overlayv1alpha1.NetworkAttachment{
				GenerateName: "shared-attachment-",
				Namespace:    key.Namespace,
				Spec: overlayv1alpha1.NetworkAttachmentSpec{
					NetworkRef:        v1alpha1.LocalObjectReference{Name: key.Name},
					InterfaceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"pod": "ap102"}},
					Encapsulation:     overlayv1alpha1.Encapsulation{Type: overlayv1alpha1.EncapsulationTypeVLAN, ID: 101},
				},
			}
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, other))).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(other), &overlayv1alpha1.NetworkAttachment{})
					g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EVPNInstance{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: other.Name})).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.VLAN{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: other.Name})).To(Succeed())
			})
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(other), na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(HaveLen(1))
				g.Expect(claims.Items[0].UID).To(Equal(claim.UID))
				g.Expect(claims.Items[0].Status.Value).To(Equal(claim.Status.Value))
				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: other.Name})).To(Succeed())
				g.Expect(evis.Items).To(HaveLen(1))
				g.Expect(strconv.FormatInt(int64(evis.Items[0].Spec.VNI), 10)).To(Equal(claim.Status.Value))
			}).Should(Succeed())
		})

		It("rejects unsupported selected Interface types", func() {
			By("Creating a matching Loopback Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-loopback-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Loopback1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the unsupported type without modifying the Interface")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.InvalidInterfaceReason),
				)))

				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport).To(BeNil())
			}).Should(Succeed())
		})

		It("recovers when a selected Interface becomes an Exact trunk", func() {
			By("Creating a matching Physical Interface without switchport configuration")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the missing switchport configuration")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.InvalidInterfaceReason),
				)))
			}).Should(Succeed())

			By("Configuring the Interface as an Exact trunk")
			current := &v1alpha1.Interface{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
			current.Spec.Switchport = &v1alpha1.Switchport{
				Mode:             v1alpha1.SwitchportModeTrunk,
				AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
			}
			Expect(k8sClient.Update(ctx, current)).To(Succeed())

			By("Recovering through the Interface update watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(na.Status.Interfaces).To(Equal([]overlayv1alpha1.NetworkAttachmentInterfaceStatus{{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
				}}))

				intf := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(current), intf)).To(Succeed())
				g.Expect(intf.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
			}).Should(Succeed())
		})

		It("rejects selected Access switchports", func() {
			By("Creating a matching Physical Interface in Access mode")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:       v1alpha1.SwitchportModeAccess,
						AccessVlan: 100,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Rejecting Access mode without changing the switchport configuration")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.InvalidInterfaceReason),
				)))

				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport).To(Equal(intf.Spec.Switchport))
			}).Should(Succeed())
		})

		It("rejects selected trunks with unmanaged allowed VLANs", func() {
			By("Creating a matching Physical trunk with unmanaged allowed VLANs")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeUnmanaged,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Rejecting Unmanaged mode without changing the switchport configuration")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.InvalidInterfaceReason),
				)))

				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport).To(Equal(intf.Spec.Switchport))
			}).Should(Succeed())
		})

		It("recovers when a referenced Device becomes available", func() {
			By("Creating a matching Interface referencing a missing Device")
			deviceName := key.Name + "-device"
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the unresolved Device reference")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.DeviceResolutionFailedReason),
				)))
			}).Should(Succeed())

			By("Creating the referenced Device with labels matching the Fabric")
			device := &v1alpha1.Device{
				Name:      deviceName,
				Namespace: key.Namespace,
				Labels:    map[string]string{"fabric-one": "true"},
				Spec: v1alpha1.DeviceSpec{
					Provider: "test-provider",
					Endpoint: v1alpha1.Endpoint{Address: "192.0.2.2:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
			})

			By("Recovering through the Device creation watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(na.Status.Interfaces).To(Equal([]overlayv1alpha1.NetworkAttachmentInterfaceStatus{{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: device.Name},
					InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
				}}))

				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
			}).Should(Succeed())
		})

		It("recovers when Device labels match a Fabric", func() {
			By("Removing the Device's Fabric membership label")
			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
				delete(device.Labels, "fabric-one")
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting that the Device does not resolve to a Fabric")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.FabricResolutionFailedReason),
				)))
			}).Should(Succeed())

			By("Restoring the Device's Fabric membership label")
			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
				if device.Labels == nil {
					device.Labels = make(map[string]string)
				}
				device.Labels["fabric-one"] = "true"
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			By("Recovering through the Device label-change watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(na.Status.ForwardingContextRef).To(Equal(&v1alpha1.TypedLocalObjectReference{
					APIVersion: evpnv1alpha1.GroupVersion.String(),
					Kind:       "Fabric",
					Name:       fabricKey.Name,
				}))

				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
			}).Should(Succeed())
		})

		It("rejects Devices selected by multiple Fabrics", func() {
			By("Creating another Fabric selecting the shared Device")
			additionalFabric := &evpnv1alpha1.Fabric{
				GenerateName: "additional-fabric-",
				Namespace:    key.Namespace,
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
						Name:       vniPoolKey.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, additionalFabric)).To(Succeed())
			additionalFabricKey := client.ObjectKeyFromObject(additionalFabric)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, additionalFabric))).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, additionalFabricKey, &evpnv1alpha1.Fabric{})
					g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: additionalFabricKey.Name})).To(Succeed())
				Eventually(func(g Gomega) {
					claims := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: additionalFabricKey.Name})).To(Succeed())
					g.Expect(claims.Items).To(BeEmpty())
				}).Should(Succeed())
			})

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting ambiguous Fabric membership")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.FabricResolutionFailedReason),
				)))
			}).Should(Succeed())

			By("Deleting the additional Fabric to restore unambiguous membership")
			Expect(k8sClient.Delete(ctx, additionalFabric)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, additionalFabricKey, &evpnv1alpha1.Fabric{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Recovering through the Fabric deletion watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(na.Status.ForwardingContextRef).To(Equal(&v1alpha1.TypedLocalObjectReference{
					APIVersion: evpnv1alpha1.GroupVersion.String(),
					Kind:       "Fabric",
					Name:       fabricKey.Name,
				}))
			}).Should(Succeed())
		})

		It("rejects allocations outside the supported VNI range", func() {
			By("Configuring the IndexPool to allocate a reserved VNI value")
			Eventually(func(g Gomega) {
				pool := &poolv1alpha1.IndexPool{}
				g.Expect(k8sClient.Get(ctx, vniPoolKey, pool)).To(Succeed())
				// The EVPNInstance API supports VNIs 1..16777214, excluding this value.
				pool.Spec.Ranges = []v1alpha1.IndexRange{{Start: 16777215, End: 16777215}}
				g.Expect(k8sClient.Update(ctx, pool)).To(Succeed())
			}).Should(Succeed())

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Waiting for the Claim controller to allocate the configured value")
			Eventually(func(g Gomega) {
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(HaveLen(1))
				g.Expect(claims.Items[0].Status.Value).To(Equal("16777215"))
				g.Expect(claims.Items[0].Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", poolv1alpha1.AllocatedCondition),
					HaveField("Status", metav1.ConditionTrue),
				)))
			}).Should(Succeed())

			By("Rejecting the allocation as an invalid VNI")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.InvalidVNIReason),
				)))
			}).Should(Succeed())
		})

		It("rejects four-octet Fabric AS numbers", func() {
			By("Configuring the Fabric with a four-octet ASN")
			Eventually(func(g Gomega) {
				fabric := &evpnv1alpha1.Fabric{}
				g.Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
				// RFC 8365 VNI-based route-target auto-derivation requires a two-octet ASN.
				fabric.Spec.Overlay.IBGP.ASNumber = intstr.FromString("65536")
				g.Expect(k8sClient.Update(ctx, fabric)).To(Succeed())
			}).Should(Succeed())

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the unsupported ASN")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.UnsupportedASNumberReason),
				)))
			}).Should(Succeed())
		})

		It("cleans up when the Network starts deletion and recovers after recreation", func() {
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						AllowedVlans:     []v1alpha1.IndexRange{{Start: 10, End: 20}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Waiting for realization and recording the original resource identities")
			net := &overlayv1alpha1.Network{}
			na := &overlayv1alpha1.NetworkAttachment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())
			Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
			networkUID := net.UID
			attachmentUID := na.UID
			claims := &poolv1alpha1.ClaimList{}
			Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
			Expect(claims.Items).To(HaveLen(1))
			claim := claims.Items[0]
			evis := &v1alpha1.EVPNInstanceList{}
			Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
			Expect(evis.Items).To(HaveLen(1))
			eviUID := evis.Items[0].UID

			By("Holding the Network in termination while its deletion update triggers cleanup")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				controllerutil.AddFinalizer(net, "test.example.com/hold-deletion")
				g.Expect(k8sClient.Update(ctx, net)).To(Succeed())
			}).Should(Succeed())
			DeferCleanup(func() {
				current := &overlayv1alpha1.Network{}
				err := k8sClient.Get(ctx, key, current)
				if apierrors.IsNotFound(err) {
					return
				}
				Expect(err).NotTo(HaveOccurred())
				if controllerutil.RemoveFinalizer(current, "test.example.com/hold-deletion") {
					Expect(k8sClient.Update(ctx, current)).To(Succeed())
				}
			})
			Expect(k8sClient.Delete(ctx, net)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.DeletionTimestamp.IsZero()).To(BeFalse())
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.UID).To(Equal(attachmentUID))
				g.Expect(na.DeletionTimestamp.IsZero()).To(BeTrue())
				g.Expect(na.Status.Interfaces).To(BeEmpty())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NetworkNotFoundReason),
				)))
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 10, End: 20}}))
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(evis.Items).To(BeEmpty())
				vlans := &v1alpha1.VLANList{}
				g.Expect(k8sClient.List(ctx, vlans, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(vlans.Items).To(BeEmpty())
			}).Should(Succeed())

			By("Releasing the allocation while keeping the Fabric and attachment")
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(&claim), &poolv1alpha1.Claim{}))).To(BeTrue())
				g.Expect(k8sClient.Get(ctx, fabricKey, &evpnv1alpha1.Fabric{})).To(Succeed())
				indices := &poolv1alpha1.IndexList{}
				g.Expect(k8sClient.List(ctx, indices, client.InNamespace(vniPoolKey.Namespace))).To(Succeed())
				g.Expect(indices.Items).NotTo(ContainElement(HaveField("Spec.PoolRef.Name", vniPoolKey.Name)))
			}).Should(Succeed())

			By("Completing Network deletion without deleting the attachment")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				controllerutil.RemoveFinalizer(net, "test.example.com/hold-deletion")
				g.Expect(k8sClient.Update(ctx, net)).To(Succeed())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.Network{}))).To(BeTrue())
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NetworkNotFoundReason),
				)))
			}).Should(Succeed())

			By("Recreating the Network and realizing the existing attachment with a fresh EVI")
			net = &overlayv1alpha1.Network{Name: key.Name, Namespace: key.Namespace}
			Expect(k8sClient.Create(ctx, net)).To(Succeed())
			Expect(net.UID).NotTo(Equal(networkUID))
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.UID).To(Equal(attachmentUID))
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 10, End: 20}, {Start: 100, End: 100}}))
				allocated := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: "vni-" + hash(net)}, allocated)).To(Succeed())
				g.Expect(allocated.UID).NotTo(Equal(claim.UID))
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(evis.Items).To(HaveLen(1))
				g.Expect(evis.Items[0].UID).NotTo(Equal(eviUID))
				g.Expect(strconv.FormatInt(int64(evis.Items[0].Spec.VNI), 10)).To(Equal(allocated.Status.Value))
			}).Should(Succeed())
		})

		It("reports a missing referenced Network", func() {
			By("Removing the original attachment before deleting its Network")
			na := &overlayv1alpha1.NetworkAttachment{}
			Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
			spec := na.Spec
			Expect(k8sClient.Delete(ctx, na)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.NetworkAttachment{}))).To(BeTrue())
			}).Should(Succeed())
			By("Deleting the referenced Network before creating an Interface")
			network := &overlayv1alpha1.Network{Name: key.Name, Namespace: key.Namespace}
			Expect(k8sClient.Delete(ctx, network)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &overlayv1alpha1.Network{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
			na = &overlayv1alpha1.NetworkAttachment{Name: key.Name, Namespace: key.Namespace, Spec: spec}
			Expect(k8sClient.Create(ctx, na)).To(Succeed())

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the unresolved Network reference")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NetworkNotFoundReason),
				)))
			}).Should(Succeed())
		})

		It("does not adopt an existing VLAN on the selected Device", func() {
			By("Creating an unowned VLAN with the requested ID")
			vlan := &v1alpha1.VLAN{
				GenerateName: "existing-vlan-",
				Namespace:    key.Namespace,
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					ID:         100,
					Name:       "existing-vlan",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vlan))).To(Succeed())
			})

			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Reporting the conflict without adopting, changing, or deleting the VLAN")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.ResourceConflictReason),
				)))

				current := &v1alpha1.VLAN{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(vlan), current)).To(Succeed())
				g.Expect(metav1.GetControllerOf(current)).To(BeNil())
				g.Expect(current.Spec).To(Equal(vlan.Spec))
				g.Expect(current.DeletionTimestamp.IsZero()).To(BeTrue())
			}).Should(Succeed())
		})

		It("recovers after a competing attachment is deleted", func() {
			By("Creating another Network and attachment requesting VLAN 100 on the same Device")
			network := &overlayv1alpha1.Network{
				GenerateName: "competing-network-",
				Namespace:    key.Namespace,
			}
			Expect(k8sClient.Create(ctx, network)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, network))).To(Succeed())
			})

			other := &overlayv1alpha1.NetworkAttachment{
				GenerateName: "competing-attachment-",
				Namespace:    key.Namespace,
				Spec: overlayv1alpha1.NetworkAttachmentSpec{
					NetworkRef: v1alpha1.LocalObjectReference{Name: network.Name},
					InterfaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"pod": "ap102"},
					},
					Encapsulation: overlayv1alpha1.Encapsulation{
						Type: overlayv1alpha1.EncapsulationTypeVLAN,
						ID:   100,
					},
				},
			}
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			otherKey := client.ObjectKeyFromObject(other)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, other))).To(Succeed())
			})

			By("Creating separately selected Exact trunk Interfaces on the shared Device")
			interfaces := []*v1alpha1.Interface{
				{
					GenerateName: "attachment-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap101"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Ethernet1/1",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypePhysical,
						Switchport: &v1alpha1.Switchport{
							Mode:             v1alpha1.SwitchportModeTrunk,
							AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						},
					},
				},
				{
					GenerateName: "competing-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap102"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Ethernet1/2",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypePhysical,
						Switchport: &v1alpha1.Switchport{
							Mode:             v1alpha1.SwitchportModeTrunk,
							AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						},
					},
				},
			}
			for _, intf := range interfaces {
				Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			}
			DeferCleanup(func() {
				for _, intf := range interfaces {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
				}
			})

			By("Reporting the encapsulation conflict on both attachments")
			Eventually(func(g Gomega) {
				for _, attachmentKey := range []client.ObjectKey{key, otherKey} {
					na := &overlayv1alpha1.NetworkAttachment{}
					g.Expect(k8sClient.Get(ctx, attachmentKey, na)).To(Succeed())
					g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
						HaveField("Type", v1alpha1.ReadyCondition),
						HaveField("Status", metav1.ConditionFalse),
						HaveField("Reason", evpnv1alpha1.EncapsulationConflictReason),
					)))
				}
			}).Should(Succeed())

			By("Deleting the competing attachment")
			Expect(k8sClient.Delete(ctx, other)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, otherKey, &overlayv1alpha1.NetworkAttachment{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Recovering through the peer deletion watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))

				intf := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[0]), intf)).To(Succeed())
				g.Expect(intf.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
			}).Should(Succeed())
		})

		It("moves VLAN membership when the Interface selector changes", func() {
			By("Creating the initially selected Interface and a replacement on the same Device")
			interfaces := []*v1alpha1.Interface{
				{
					GenerateName: "initial-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap101"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Ethernet1/1",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypePhysical,
						Switchport: &v1alpha1.Switchport{
							Mode:             v1alpha1.SwitchportModeTrunk,
							AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						},
					},
				},
				{
					GenerateName: "replacement-interface-",
					Namespace:    key.Namespace,
					Labels:       map[string]string{"pod": "ap102"},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
						Name:       "Ethernet1/2",
						AdminState: v1alpha1.AdminStateUp,
						Type:       v1alpha1.InterfaceTypePhysical,
						Switchport: &v1alpha1.Switchport{
							Mode:             v1alpha1.SwitchportModeTrunk,
							AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						},
					},
				},
			}
			for _, intf := range interfaces {
				Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			}
			DeferCleanup(func() {
				for _, intf := range interfaces {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
				}
			})

			By("Waiting for the initial Interface membership to be realized and recorded")
			Eventually(func(g Gomega) {
				intf := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[0]), intf)).To(Succeed())
				g.Expect(intf.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Interfaces).To(Equal([]overlayv1alpha1.NetworkAttachmentInterfaceStatus{{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					InterfaceRef: v1alpha1.LocalObjectReference{Name: interfaces[0].Name},
				}}))
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())

			By("Changing the attachment selector to the replacement Interface")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				na.Spec.InterfaceSelector = metav1.LabelSelector{MatchLabels: map[string]string{"pod": "ap102"}}
				g.Expect(k8sClient.Update(ctx, na)).To(Succeed())
			}).Should(Succeed())

			By("Cleaning the old membership and recording only the replacement")
			Eventually(func(g Gomega) {
				initial := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[0]), initial)).To(Succeed())
				g.Expect(initial.Spec.Switchport.AllowedVlans).To(BeNil())
				replacement := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(interfaces[1]), replacement)).To(Succeed())
				g.Expect(replacement.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Interfaces).To(Equal([]overlayv1alpha1.NetworkAttachmentInterfaceStatus{{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					InterfaceRef: v1alpha1.LocalObjectReference{Name: interfaces[1].Name},
				}}))
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
			}).Should(Succeed())
		})

		It("cleans up realization when the Interface selector matches nothing", func() {
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
						AllowedVlans:     []v1alpha1.IndexRange{{Start: 10, End: 20}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Waiting for Interface membership and device resources to be realized")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 10, End: 20}, {Start: 100, End: 100}}))
				vlans := &v1alpha1.VLANList{}
				g.Expect(k8sClient.List(ctx, vlans, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(vlans.Items).To(HaveLen(1))
				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(evis.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Changing the selector to match no Interfaces")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				na.Spec.InterfaceSelector = metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}}
				g.Expect(k8sClient.Update(ctx, na)).To(Succeed())
			}).Should(Succeed())

			By("Removing only owned membership and deleting EVIs before VLANs")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.DeletionTimestamp.IsZero()).To(BeTrue())
				g.Expect(na.Status.Interfaces).To(BeEmpty())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", evpnv1alpha1.NoMatchingInterfacesReason),
				)))
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 10, End: 20}}))
				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(evis.Items).To(BeEmpty())
				vlans := &v1alpha1.VLANList{}
				g.Expect(k8sClient.List(ctx, vlans, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: key.Name})).To(Succeed())
				g.Expect(vlans.Items).To(BeEmpty())
			}).Should(Succeed())
		})

		It("removes Interface VLAN membership when the attachment is deleted", func() {
			By("Creating a matching Physical Exact trunk Interface")
			intf := &v1alpha1.Interface{
				GenerateName: "attachment-interface-",
				Namespace:    key.Namespace,
				Labels:       map[string]string{"pod": "ap101"},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					Switchport: &v1alpha1.Switchport{
						Mode:             v1alpha1.SwitchportModeTrunk,
						AllowedVlansMode: v1alpha1.AllowedVlansModeExact,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			})

			By("Waiting for successful realization and recording the retained VNI Claim")
			var claimKey client.ObjectKey
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, key, na)).To(Succeed())
				g.Expect(na.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				g.Expect(na.Status.Interfaces).To(Equal([]overlayv1alpha1.NetworkAttachmentInterfaceStatus{{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
				}}))
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(Equal([]v1alpha1.IndexRange{{Start: 100, End: 100}}))
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(HaveLen(1))
				g.Expect(claims.Items[0].Status.Value).NotTo(BeEmpty())
				claimKey = client.ObjectKeyFromObject(&claims.Items[0])
			}).Should(Succeed())

			By("Deleting the attachment and waiting for finalizer cleanup")
			na := &overlayv1alpha1.NetworkAttachment{Name: key.Name, Namespace: key.Namespace}
			Expect(k8sClient.Delete(ctx, na)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &overlayv1alpha1.NetworkAttachment{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				current := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), current)).To(Succeed())
				g.Expect(current.Spec.Switchport.AllowedVlans).To(BeNil())
				claim := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, claimKey, claim)).To(Succeed())
				g.Expect(claim.DeletionTimestamp.IsZero()).To(BeTrue())
				net := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, net)).To(Succeed())
				g.Expect(net.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason),
				)))
				fabric := &evpnv1alpha1.Fabric{}
				g.Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
				g.Expect(metav1.IsControlledBy(claim, fabric)).To(BeTrue())
			}).Should(Succeed())
		})
	})
})
