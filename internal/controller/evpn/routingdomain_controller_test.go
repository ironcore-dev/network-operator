// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("RoutingDomain Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			key, deviceKey, fabricKey, poolKey                                 client.ObjectKey
			networkKeys, attachmentKeys, interfaceKeys, deviceKeys, fabricKeys []client.ObjectKey
		)

		addAttachment := func(deviceKey client.ObjectKey, vlan int32) {
			net := &overlayv1alpha1.Network{
				GenerateName: "routing-network-", Namespace: key.Namespace,
				Spec: overlayv1alpha1.NetworkSpec{RoutingDomainRef: &v1alpha1.LocalObjectReference{Name: key.Name}},
			}
			Expect(k8sClient.Create(ctx, net)).To(Succeed())
			networkKeys = append(networkKeys, client.ObjectKeyFromObject(net))
			intf := &v1alpha1.Interface{
				GenerateName: "routing-interface-", Namespace: key.Namespace,
				Labels: map[string]string{"routing-network": net.Name},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceKey.Name},
					Name:      fmt.Sprintf("Ethernet1/%d", vlan), Type: v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
					Switchport: &v1alpha1.Switchport{Mode: v1alpha1.SwitchportModeTrunk, AllowedVlansMode: v1alpha1.AllowedVlansModeExact},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			interfaceKeys = append(interfaceKeys, client.ObjectKeyFromObject(intf))
			na := &overlayv1alpha1.NetworkAttachment{
				GenerateName: "routing-attachment-", Namespace: key.Namespace,
				Spec: overlayv1alpha1.NetworkAttachmentSpec{
					NetworkRef:        v1alpha1.LocalObjectReference{Name: net.Name},
					InterfaceSelector: metav1.LabelSelector{MatchLabels: intf.Labels},
					Encapsulation:     overlayv1alpha1.Encapsulation{Type: overlayv1alpha1.EncapsulationTypeVLAN, ID: vlan},
				},
			}
			Expect(k8sClient.Create(ctx, na)).To(Succeed())
			attachmentKeys = append(attachmentKeys, client.ObjectKeyFromObject(na))
		}

		waitReady := func() (*overlayv1alpha1.RoutingDomain, *poolv1alpha1.Claim) {
			rd := &overlayv1alpha1.RoutingDomain{}
			claim := &poolv1alpha1.Claim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ReadyReason), HaveField("ObservedGeneration", rd.Generation),
				)))
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: "vni-" + hash(rd)}, claim)).To(Succeed())
				g.Expect(claim.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", poolv1alpha1.AllocatedCondition), HaveField("Status", metav1.ConditionTrue),
				)))
				g.Expect(claim.Status.Value).NotTo(BeEmpty())
				g.Expect(claim.Status.AllocationRef).NotTo(BeNil())
			}).Should(Succeed())
			return rd, claim
		}

		addFabric := func(selector metav1.LabelSelector) *evpnv1alpha1.Fabric {
			original := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, original)).To(Succeed())
			fabric := &evpnv1alpha1.Fabric{
				GenerateName: "other-routing-fabric-", Namespace: key.Namespace,
				Spec: original.Spec,
			}
			fabric.Spec.DeviceSelector = selector
			Expect(k8sClient.Create(ctx, fabric)).To(Succeed())
			fabricKeys = append(fabricKeys, client.ObjectKeyFromObject(fabric))
			return fabric
		}

		BeforeEach(func() {
			networkKeys, attachmentKeys, interfaceKeys = nil, nil, nil
			deviceKeys = make([]client.ObjectKey, 0, 2)
			fabricKeys = make([]client.ObjectKey, 0, 2)
			rd := &overlayv1alpha1.RoutingDomain{
				GenerateName: "test-routing-domain-", Namespace: metav1.NamespaceDefault,
				Spec: overlayv1alpha1.RoutingDomainSpec{Name: "TEST"},
			}
			Expect(k8sClient.Create(ctx, rd)).To(Succeed())
			key = client.ObjectKeyFromObject(rd)
			pool := &poolv1alpha1.IndexPool{
				GenerateName: "routing-vni-pool-", Namespace: key.Namespace,
				Spec: poolv1alpha1.IndexPoolSpec{Ranges: []v1alpha1.IndexRange{v1alpha1.MustParseIndexRange("20000..20010")}},
			}
			Expect(k8sClient.Create(ctx, pool)).To(Succeed())
			poolKey = client.ObjectKeyFromObject(pool)
			device := &v1alpha1.Device{
				GenerateName: "routing-device-", Namespace: key.Namespace,
				Labels: map[string]string{"routing-fabric": key.Name},
				Spec:   v1alpha1.DeviceSpec{Provider: "test-provider", Endpoint: v1alpha1.Endpoint{Address: "192.0.2.1:9339"}},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceKey = client.ObjectKeyFromObject(device)
			deviceKeys = append(deviceKeys, deviceKey)
			fabric := &evpnv1alpha1.Fabric{
				GenerateName: "routing-fabric-", Namespace: key.Namespace,
				Spec: evpnv1alpha1.FabricSpec{
					DeviceSelector: metav1.LabelSelector{MatchLabels: device.Labels},
					Loopbacks:      evpnv1alpha1.FabricLoopbacksSpec{IPAddressPoolRef: v1alpha1.LocalObjectReference{Name: "unused-loopback-pool"}},
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
								Name: "unused", DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
								ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
							}},
						},
					},
					BUM: evpnv1alpha1.FabricBUMSpec{
						Type: evpnv1alpha1.BUMTypeMulticast,
						PIM: &evpnv1alpha1.FabricPIMSpec{AnycastRendezvousPoints: []evpnv1alpha1.AnycastRendezvousPoint{{
							Name: "unused", MulticastGroups: []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix("224.0.0.0/4")},
							DeviceSelector:       metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
							ClientDeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}},
						}}},
					},
					VTEP:    evpnv1alpha1.FabricVTEPSpec{DeviceSelector: metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}}},
					VNIPool: v1alpha1.TypedLocalObjectReference{APIVersion: poolv1alpha1.GroupVersion.String(), Kind: "IndexPool", Name: pool.Name},
				},
			}
			Expect(k8sClient.Create(ctx, fabric)).To(Succeed())
			fabricKey = client.ObjectKeyFromObject(fabric)
			fabricKeys = append(fabricKeys, fabricKey)
		})

		AfterEach(func() {
			for _, attachmentKey := range attachmentKeys {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &overlayv1alpha1.NetworkAttachment{Name: attachmentKey.Name, Namespace: attachmentKey.Namespace}))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, attachmentKey, &overlayv1alpha1.NetworkAttachment{}))).To(BeTrue())
				}).Should(Succeed())
				// Envtest does not run Kubernetes garbage collection.
				options := []client.DeleteAllOfOption{client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkAttachmentLabel: attachmentKey.Name}}
				Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EVPNInstance{}, options...)).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Interface{}, options...)).To(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.VLAN{}, options...)).To(Succeed())
			}
			for _, networkKey := range networkKeys {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &overlayv1alpha1.Network{Name: networkKey.Name, Namespace: networkKey.Namespace}))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, networkKey, &overlayv1alpha1.Network{}))).To(BeTrue())
				}).Should(Succeed())
			}
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &overlayv1alpha1.RoutingDomain{Name: key.Name, Namespace: key.Namespace}))).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.RoutingDomain{}))).To(BeTrue())
			}).Should(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EVPNInstance{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.VRF{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
			for _, interfaceKey := range interfaceKeys {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &v1alpha1.Interface{Name: interfaceKey.Name, Namespace: interfaceKey.Namespace}))).To(Succeed())
			}
			for _, fabricKey := range fabricKeys {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &evpnv1alpha1.Fabric{Name: fabricKey.Name, Namespace: fabricKey.Namespace}))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, fabricKey, &evpnv1alpha1.Fabric{}))).To(BeTrue())
				}).Should(Succeed())
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: fabricKey.Name})).To(Succeed())
				Eventually(func(g Gomega) {
					claims := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.FabricLabel: fabricKey.Name})).To(Succeed())
					g.Expect(claims.Items).To(BeEmpty())
				}).Should(Succeed())
			}
			Eventually(func(g Gomega) {
				indices := &poolv1alpha1.IndexList{}
				g.Expect(k8sClient.List(ctx, indices, client.InNamespace(key.Namespace))).To(Succeed())
				g.Expect(indices.Items).NotTo(ContainElement(HaveField("Spec.PoolRef.Name", poolKey.Name)))
			}).Should(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &poolv1alpha1.IndexPool{Name: poolKey.Name, Namespace: poolKey.Namespace}))).To(Succeed())
			for _, deviceKey := range deviceKeys {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &v1alpha1.Device{Name: deviceKey.Name, Namespace: deviceKey.Namespace}))).To(Succeed())
			}
		})

		It("shares one VNI and one routing context per Device across Networks", func() {
			By("Reporting an unused RoutingDomain without allocating a Claim")
			Eventually(func(g Gomega) {
				rd := &overlayv1alpha1.RoutingDomain{}
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(rd, evpnv1alpha1.FinalizerName)).To(BeTrue())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.NoResourcesReason),
				)))
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(BeEmpty())
			}).Should(Succeed())

			By("Attaching two Networks to the same Device and a third to another Device")
			addAttachment(deviceKey, 100)
			addAttachment(deviceKey, 101)
			device := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
			other := &v1alpha1.Device{
				GenerateName: "shared-routing-device-", Namespace: key.Namespace, Labels: device.Labels,
				Spec: device.Spec,
			}
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			deviceKeys = append(deviceKeys, client.ObjectKeyFromObject(other))
			addAttachment(client.ObjectKeyFromObject(other), 102)
			rd, claim := waitReady()
			fabric := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
			vni, err := strconv.ParseInt(claim.Status.Value, 10, 32)
			Expect(err).NotTo(HaveOccurred())
			routeTarget := fmt.Sprintf("65000:%d", int64(1<<28)|vni)

			By("Checking allocation identity, pool and both owners")
			claims := &poolv1alpha1.ClaimList{}
			Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: rd.Name})).To(Succeed())
			Expect(claims.Items).To(HaveLen(1))
			Expect(claim.Name).To(Equal("vni-" + hash(rd)))
			Expect(claim.Spec.PoolRef).To(Equal(fabric.Spec.VNIPool))
			Expect(claim.OwnerReferences).To(ContainElement(SatisfyAll(HaveField("Kind", "RoutingDomain"), HaveField("UID", rd.UID))))
			Expect(metav1.IsControlledBy(claim, fabric)).To(BeTrue())

			By("Creating only one shared VRF and routed EVPNInstance per Device")
			vrfs := &v1alpha1.VRFList{}
			evis := &v1alpha1.EVPNInstanceList{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, vrfs, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: rd.Name})).To(Succeed())
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: rd.Name})).To(Succeed())
				g.Expect(vrfs.Items).To(HaveLen(2))
				g.Expect(evis.Items).To(HaveLen(2))
				for i := range vrfs.Items {
					vrf := &vrfs.Items[i]
					g.Expect(vrf.Spec.Name).To(Equal(rd.Spec.Name))
					g.Expect(vrf.Spec.RouteDistinguisher).To(Equal("Auto"))
					g.Expect(vrf.Spec.RouteTargets).To(Equal([]v1alpha1.RouteTarget{{Value: routeTarget, AddressFamilies: []v1alpha1.RouteTargetAF{v1alpha1.IPv4, v1alpha1.IPv4EVPN}, Action: v1alpha1.RouteTargetActionBoth}}))
					g.Expect(metav1.IsControlledBy(vrf, rd)).To(BeTrue())
					g.Expect(vrf.Spec.DeviceRef.Name).To(BeElementOf(device.Name, other.Name))
					g.Expect(vrf.Labels).To(HaveKeyWithValue(evpnv1alpha1.FabricLabel, fabric.Name))
				}
				for i := range evis.Items {
					evi := &evis.Items[i]
					g.Expect(evi.Spec.Type).To(Equal(v1alpha1.EVPNInstanceTypeRouted))
					g.Expect(evi.Spec.VNI).To(Equal(int32(vni)))
					g.Expect(metav1.IsControlledBy(evi, rd)).To(BeTrue())
					g.Expect(evi.Spec.VRFRef).NotTo(BeNil())
					g.Expect(vrfs.Items).To(ContainElement(SatisfyAll(HaveField("Name", evi.Spec.VRFRef.Name), HaveField("Spec.DeviceRef.Name", evi.Spec.DeviceRef.Name))))
				}
			}).Should(Succeed())

			By("Leaving Claim and routing resources untouched on repeated reconciliation")
			r := &RoutingDomainReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			resources := make([]client.Object, 0, 1+len(vrfs.Items)+len(evis.Items))
			resources = append(resources, claim)
			for i := range vrfs.Items {
				resources = append(resources, &vrfs.Items[i])
			}
			for i := range evis.Items {
				resources = append(resources, &evis.Items[i])
			}
			versions := make([]string, len(resources))
			for i, resource := range resources {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), resource)).To(Succeed())
				versions[i] = resource.GetResourceVersion()
			}
			current, err := r.reconcileVNIClaim(ctx, rd, fabric)
			Expect(err).NotTo(HaveOccurred())
			Expect(current).NotTo(BeNil())
			Expect(r.reconcileRoutingContext(ctx, rd, map[string]*v1alpha1.Device{device.Name: device, other.Name: other}, fabric, uint64(vni), routeTarget)).To(Succeed())
			for i, resource := range resources {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), resource)).To(Succeed())
				Expect(resource.GetResourceVersion()).To(Equal(versions[i]))
			}
		})

		It("cleans up unused routing resources and releases the VNI on deletion", func() {
			addAttachment(deviceKey, 100)
			rd, claim := waitReady()
			allocationKey := client.ObjectKey{Namespace: key.Namespace, Name: claim.Status.AllocationRef.Name}
			By("Removing the last selected Device through the attachment selector watch")
			Eventually(func(g Gomega) {
				na := &overlayv1alpha1.NetworkAttachment{}
				g.Expect(k8sClient.Get(ctx, attachmentKeys[0], na)).To(Succeed())
				na.Spec.InterfaceSelector = metav1.LabelSelector{MatchLabels: map[string]string{"never": "match"}}
				g.Expect(k8sClient.Update(ctx, na)).To(Succeed())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.NoResourcesReason))))
				vrfs := &v1alpha1.VRFList{}
				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sClient.List(ctx, vrfs, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
				g.Expect(vrfs.Items).To(BeEmpty())
				g.Expect(evis.Items).To(BeEmpty())
			}).Should(Succeed())
			By("Retaining the allocation for the lifetime of the RoutingDomain")
			current := &poolv1alpha1.Claim{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), current)).To(Succeed())
			Expect(current.UID).To(Equal(claim.UID))
			Expect(current.Status.Value).To(Equal(claim.Status.Value))
			By("Finalizing the Claim and recycling its Index when the RoutingDomain is deleted")
			Expect(k8sClient.Delete(ctx, rd)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &overlayv1alpha1.RoutingDomain{}))).To(BeTrue())
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))).To(BeTrue())
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, allocationKey, &poolv1alpha1.Index{}))).To(BeTrue())
			}).Should(Succeed())
		})

		It("rejects ambiguous Fabric selection and recovers after deletion", func() {
			fabric := &evpnv1alpha1.Fabric{}
			Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
			other := addFabric(fabric.Spec.DeviceSelector)
			addAttachment(deviceKey, 100)
			Eventually(func(g Gomega) {
				rd := &overlayv1alpha1.RoutingDomain{}
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.FabricResolutionFailedReason))))
				claims := &poolv1alpha1.ClaimList{}
				g.Expect(k8sClient.List(ctx, claims, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
				g.Expect(claims.Items).To(BeEmpty())
			}).Should(Succeed())
			By("Recovering through the Fabric deletion watch")
			Expect(k8sClient.Delete(ctx, other)).To(Succeed())
			waitReady()
		})

		It("preserves its VNI when selected Devices move to another Fabric", func() {
			addAttachment(deviceKey, 100)
			_, claim := waitReady()
			other := addFabric(metav1.LabelSelector{MatchLabels: map[string]string{"other-routing-fabric": key.Name}})
			By("Moving the Device without rebinding the existing Claim")
			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, deviceKey, device)).To(Succeed())
				device.Labels = other.Spec.DeviceSelector.MatchLabels
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				rd := &overlayv1alpha1.RoutingDomain{}
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.StretchedNetworkNotSupportedReason))))
				current := &poolv1alpha1.Claim{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), current)).To(Succeed())
				g.Expect(current.UID).To(Equal(claim.UID))
				g.Expect(current.Spec).To(Equal(claim.Spec))
				g.Expect(current.OwnerReferences).To(Equal(claim.OwnerReferences))
				g.Expect(current.Status.Value).To(Equal(claim.Status.Value))
			}).Should(Succeed())
		})

		It("rejects unsupported Fabric AS numbers", func() {
			Eventually(func(g Gomega) {
				fabric := &evpnv1alpha1.Fabric{}
				g.Expect(k8sClient.Get(ctx, fabricKey, fabric)).To(Succeed())
				fabric.Spec.Overlay.IBGP.ASNumber = intstr.FromString("65536")
				g.Expect(k8sClient.Update(ctx, fabric)).To(Succeed())
			}).Should(Succeed())
			addAttachment(deviceKey, 100)
			Eventually(func(g Gomega) {
				rd := &overlayv1alpha1.RoutingDomain{}
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.UnsupportedASNumberReason))))
			}).Should(Succeed())
		})

		It("rejects allocations outside the supported VNI range", func() {
			Eventually(func(g Gomega) {
				pool := &poolv1alpha1.IndexPool{}
				g.Expect(k8sClient.Get(ctx, poolKey, pool)).To(Succeed())
				pool.Spec.Ranges = []v1alpha1.IndexRange{{Start: 16777215, End: 16777215}}
				g.Expect(k8sClient.Update(ctx, pool)).To(Succeed())
			}).Should(Succeed())
			addAttachment(deviceKey, 100)
			Eventually(func(g Gomega) {
				rd := &overlayv1alpha1.RoutingDomain{}
				g.Expect(k8sClient.Get(ctx, key, rd)).To(Succeed())
				g.Expect(rd.Status.Conditions).To(ContainElement(SatisfyAll(HaveField("Type", v1alpha1.ReadyCondition), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", evpnv1alpha1.InvalidVNIReason))))
				evis := &v1alpha1.EVPNInstanceList{}
				g.Expect(k8sClient.List(ctx, evis, client.InNamespace(key.Namespace), client.MatchingLabels{evpnv1alpha1.RoutingDomainLabel: key.Name})).To(Succeed())
				g.Expect(evis.Items).To(BeEmpty())
			}).Should(Succeed())
		})
	})
})
