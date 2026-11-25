// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxoverlay

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nxoverlayv1alpha1 "github.com/ironcore-dev/network-operator/api/nx.overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

var _ = Describe("EVPNInterconnect Controller", func() {
	var r *EVPNInterconnectReconciler
	var ic *nxoverlayv1alpha1.EVPNInterconnect
	var state *ReconcileState
	var recorded *interconnectRecordingClient
	BeforeEach(func() {
		ic = testInterconnect()
		recorded = &interconnectRecordingClient{Client: fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).
			WithStatusSubresource(&poolv1alpha1.Claim{}, &nxoverlayv1alpha1.EVPNInterconnect{}).WithObjects(ic).Build()}
		r = &EVPNInterconnectReconciler{Client: recorded, Scheme: k8sClient.Scheme()}
		state = &ReconcileState{vteps: make(map[string][]corev1alpha1.NetworkVirtualizationEdge), multiSiteID: 123, devices: map[string]*corev1alpha1.Device{
			"device-1": {Name: "device-1", Spec: corev1alpha1.DeviceSpec{Provider: "interconnect-test-nxos"}},
			"device-2": {Name: "device-2", Spec: corev1alpha1.DeviceSpec{Provider: "interconnect-test-nxos"}},
		}}
	})
	Context("When reconciling the lifecycle", func() {
		It("initializes conditions and finalizes through the API server", func() {
			ic.UID, ic.ResourceVersion = "", ""
			Expect(k8sClient.Create(ctx, ic)).To(Succeed())
			DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ic))).To(Succeed()) })
			key := client.ObjectKeyFromObject(ic)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, ic)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName)).To(BeTrue())
				g.Expect(conditions.Get(ic, corev1alpha1.ReadyCondition)).NotTo(BeNil())
			}).Should(Succeed())
			Expect(k8sClient.Delete(ctx, ic)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, key, ic))
			}).Should(BeTrue())
		})
		It("rejects changing the loopback address pool through the API server", func() {
			ic.UID, ic.ResourceVersion = "", ""
			ic.Name = "immutable-interconnect"
			Expect(k8sClient.Create(ctx, ic)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, ic)).To(Succeed()) })
			changed := ic.DeepCopy()
			changed.Spec.IPAddressPoolRef.Name = "other-loopbacks"
			err := k8sClient.Update(ctx, changed)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err).To(MatchError(ContainSubstring("ipAddressPoolRef is immutable")))
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ic), ic); err != nil {
					return err
				}
				return k8sClient.Update(ctx, ic)
			}).Should(Succeed())
		})
	})

	Context("When reconciling loopbacks", func() {
		It("allocates per-device loopbacks and repairs drift without redundant patches", func() {
			_, err := r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(state.loopbacks).To(BeEmpty())
			for i := 1; i <= 2; i++ {
				allocateInterconnectClaim(r.Client, fmt.Sprintf("interconnect-device-%d-lo9", i), fmt.Sprintf("10.0.0.%d", i))
			}
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(state.loopbacks).To(HaveLen(2))
			for name, intf := range state.loopbacks {
				Expect(intf.Spec.Name).To(Equal("Loopback9"))
				Expect(intf.Spec.DeviceRef.Name).To(Equal(name))
				Expect(intf.Spec.Type).To(Equal(corev1alpha1.InterfaceTypeLoopback))
				Expect(intf.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
				Expect(intf.Spec.IPv4.Addresses[0].Bits()).To(Equal(32))
				Expect(metav1.IsControlledBy(intf, ic)).To(BeTrue())
				claim := &poolv1alpha1.Claim{}
				Expect(r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: intf.Name}, claim)).To(Succeed())
				Expect(metav1.IsControlledBy(claim, ic)).To(BeTrue())
			}
			patches := recorded.patches
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.patches).To(Equal(patches))
			intf := state.loopbacks["device-1"]
			intf.Spec.AdminState = corev1alpha1.AdminStateDown
			intf.Spec.IPv4.Addresses = []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.9.9.9/32")}
			intf.Labels = map[string]string{"unrelated": "keep"}
			Expect(r.Update(ctx, intf)).To(Succeed())
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(state.loopbacks["device-1"].Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
			Expect(state.loopbacks["device-1"].Spec.IPv4.Addresses[0].String()).To(Equal("10.0.0.1/32"))
			Expect(state.loopbacks["device-1"].Labels).To(HaveKeyWithValue("unrelated", "keep"))
			patches = recorded.patches
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.patches).To(Equal(patches))
		})
		DescribeTable("rejects invalid loopback allocations", func(value string) {
			_, err := r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			allocateInterconnectClaim(r.Client, "interconnect-device-1-lo9", value)
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())
			Expect(state.loopbacks).To(BeEmpty())
		}, Entry("malformed address", "invalid"), Entry("IPv6 address", "2001:db8::1"))
	})

	Context("When reconciling border gateways", func() {
		BeforeEach(func() {
			prepareInterconnectInterfaces(r, ic, state)
			for deviceName := range state.devices {
				nve := &corev1alpha1.NetworkVirtualizationEdge{
					Name: "fabric-" + deviceName, Namespace: ic.Namespace, Generation: 2,
					Spec: corev1alpha1.NetworkVirtualizationEdgeSpec{DeviceRef: corev1alpha1.LocalObjectReference{Name: deviceName}},
				}
				nve.Status.Conditions = []metav1.Condition{{Type: corev1alpha1.ConfiguredCondition, Status: metav1.ConditionTrue, ObservedGeneration: nve.Generation}}
				state.vteps[nve.Spec.DeviceRef.Name] = []corev1alpha1.NetworkVirtualizationEdge{*nve}
			}
		})

		DescribeTable("waits for dependencies or rejects ambiguous NVEs", func(prepare func(), terminal bool) {
			prepare()
			res, err := r.reconcileBorderGateway(ctx, ic, state)
			if terminal {
				Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(dependencyRetryInterval))
			bgw := &nxv1alpha1.BorderGateway{}
			Expect(apierrors.IsNotFound(r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: "interconnect-device-1-bgw"}, bgw))).To(BeTrue())
		},
			Entry("unallocated multisite ID", func() { state.multiSiteID = 0 }, false),
			Entry("missing NVE", func() { delete(state.vteps, "device-1") }, false),
			Entry("stale NVE status", func() { state.vteps["device-1"][0].Status.Conditions[0].ObservedGeneration-- }, false),
			Entry("deleting NVE", func() { now := metav1.Now(); state.vteps["device-1"][0].DeletionTimestamp = &now }, false),
			Entry("missing loopback", func() { delete(state.loopbacks, "device-1") }, false),
			Entry("missing DCI interface", func() { delete(state.interfaces, "dci-1") }, false),
			Entry("deleting DCI interface", func() { now := metav1.Now(); state.interfaces["dci-1"].DeletionTimestamp = &now }, false),
			Entry("multiple NVEs on a device", func() {
				duplicate := state.vteps["device-1"][0].DeepCopy()
				duplicate.Name = "duplicate"
				state.vteps["device-1"] = append(state.vteps["device-1"], *duplicate)
			}, true),
		)

		It("creates an owned BorderGateway per device and reconciles interconnect fields", func() {
			state.vteps["unrelated-device"] = []corev1alpha1.NetworkVirtualizationEdge{{Name: "unrelated"}}
			res, err := r.reconcileBorderGateway(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())
			bgws := &nxv1alpha1.BorderGatewayList{}
			Expect(r.List(ctx, bgws)).To(Succeed())
			Expect(bgws.Items).To(HaveLen(2))
			for i := range bgws.Items {
				bgw := &bgws.Items[i]
				Expect(metav1.IsControlledBy(bgw, ic)).To(BeTrue())
				Expect(bgw.Spec.MultisiteID).To(Equal(int64(123)))
				Expect(bgw.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
				Expect(bgw.Spec.DelayRestoreTime.Duration).To(Equal(delayRestoreTime))
				Expect(bgw.Spec.SourceInterfaceRef.Name).To(Equal(state.loopbacks[bgw.Spec.DeviceRef.Name].Name))
				Expect(bgw.Spec.InterconnectInterfaceRefs).To(HaveLen(1))
				Expect(bgw.Spec.InterconnectInterfaceRefs[0].Tracking).To(Equal(nxv1alpha1.InterconnectTrackingTypeDCI))
				dci := &corev1alpha1.Interface{}
				Expect(r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: bgw.Spec.InterconnectInterfaceRefs[0].Name}, dci)).To(Succeed())
				Expect(dci.Spec.DeviceRef).To(Equal(bgw.Spec.DeviceRef))
			}
			patches := recorded.patches
			_, err = r.reconcileBorderGateway(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.patches).To(Equal(patches))
			bgw := &bgws.Items[0]
			bgw.Spec.MultisiteID = 456
			bgw.Spec.AdminState = corev1alpha1.AdminStateDown
			bgw.Spec.DelayRestoreTime = metav1.Duration{Duration: 60 * time.Second}
			bgw.Spec.BGPPeerRefs = []nxv1alpha1.BGPPeerReference{{Name: "peer", PeerType: nxv1alpha1.BGPPeerTypeFabricExternal}}
			bgw.Spec.StormControl = []nxv1alpha1.StormControl{{Traffic: nxv1alpha1.TrafficTypeBroadcast, Level: "1"}}
			bgw.Labels = map[string]string{"unrelated": "keep"}
			Expect(r.Update(ctx, bgw)).To(Succeed())
			_, err = r.reconcileBorderGateway(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Get(ctx, client.ObjectKeyFromObject(bgw), bgw)).To(Succeed())
			Expect(bgw.Spec.MultisiteID).To(Equal(int64(123)))
			Expect(bgw.Spec.AdminState).To(Equal(corev1alpha1.AdminStateUp))
			Expect(bgw.Spec.DelayRestoreTime.Duration).To(Equal(60 * time.Second))
			Expect(bgw.Spec.BGPPeerRefs).To(HaveLen(1))
			Expect(bgw.Spec.StormControl).To(HaveLen(1))
			Expect(bgw.Labels).To(HaveKeyWithValue("unrelated", "keep"))
			ic.Spec.Interfaces = ic.Spec.Interfaces[:1]
			_, err = r.reconcileBorderGateway(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			bgw2 := &nxv1alpha1.BorderGateway{}
			Expect(r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: "interconnect-device-2-bgw"}, bgw2)).To(Succeed())
			Expect(bgw2.Spec.InterconnectInterfaceRefs).To(BeEmpty())
		})
	})

	DescribeTable("watches dependencies only in the interconnect namespace", func(dependency client.Object) {
		other := testInterconnect()
		other.Name, other.Namespace = "other", "other-namespace"
		Expect(r.Create(ctx, other)).To(Succeed())
		dependency.SetNamespace(ic.Namespace)
		Expect(r.dependenciesToInterconnects(ctx, dependency)).To(Equal([]reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(ic)}}))
	},
		Entry("Interfaces", &corev1alpha1.Interface{}),
		Entry("NVEs", &corev1alpha1.NetworkVirtualizationEdge{}),
	)

	Context("When cleaning up owned resources", func() {
		It("cleans dependents before loopbacks and Claims and retries failures", func() {
			bgw := &nxv1alpha1.BorderGateway{Name: "bgw", Namespace: ic.Namespace, Finalizers: []string{"hold"}}
			intf := &corev1alpha1.Interface{Name: "loopback", Namespace: ic.Namespace, Finalizers: []string{"hold"}}
			claim := &poolv1alpha1.Claim{Name: "claim", Namespace: ic.Namespace}
			for _, obj := range []client.Object{bgw, intf, claim} {
				Expect(controllerutil.SetControllerReference(ic, obj, r.Scheme)).To(Succeed())
				Expect(r.Create(ctx, obj)).To(Succeed())
			}
			foreign := &corev1alpha1.Interface{Name: "foreign", Namespace: ic.Namespace}
			Expect(r.Create(ctx, foreign)).To(Succeed())
			fabricNVE := &corev1alpha1.NetworkVirtualizationEdge{Name: "fabric-nve", Namespace: ic.Namespace}
			Expect(r.Create(ctx, fabricNVE)).To(Succeed())
			recorded.deleteError = errors.New("device cleanup unavailable")
			_, err := r.finalize(ctx, ic)
			Expect(err).To(MatchError(ContainSubstring("device cleanup unavailable")))
			recorded.deleteError, recorded.deletes = nil, nil
			res, err := r.finalize(ctx, ic)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(dependencyRetryInterval))
			Expect(recorded.deletes).To(Equal([]string{"bgw"}))
			res, err = r.finalize(ctx, ic)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(dependencyRetryInterval))
			Expect(r.Get(ctx, client.ObjectKeyFromObject(claim), claim)).To(Succeed())
			completeInterconnectChildDeletion(r.Client, bgw)
			_, err = r.finalize(ctx, ic)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.deletes).To(Equal([]string{"bgw", "loopback"}))
			Expect(r.Get(ctx, client.ObjectKeyFromObject(claim), claim)).To(Succeed())
			completeInterconnectChildDeletion(r.Client, intf)
			_, err = r.finalize(ctx, ic)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.deletes).To(Equal([]string{"bgw", "loopback", "claim"}))
			res, err = r.finalize(ctx, ic)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())
			Expect(r.Get(ctx, client.ObjectKeyFromObject(foreign), foreign)).To(Succeed())
			Expect(r.Get(ctx, client.ObjectKeyFromObject(fabricNVE), fabricNVE)).To(Succeed())
		})
		It("cleans removed members without treating unpopulated discovery as empty membership", func() {
			_, err := r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			allocateInterconnectClaim(r.Client, "interconnect-device-1-lo9", "10.0.0.1")
			_, err = r.reconcileLoopbacks(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			_, err = r.reconcileLoopbacks(ctx, ic, &ReconcileState{})
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.deletes).To(BeEmpty())
			delete(state.devices, "device-1")
			for range 3 {
				_, err = r.reconcileLoopbacks(ctx, ic, state)
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(recorded.deletes).To(Equal([]string{"interconnect-device-1-lo9", "interconnect-device-1-lo9"}))
			claim := &poolv1alpha1.Claim{}
			Expect(r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: "interconnect-device-2-lo9"}, claim)).To(Succeed())
		})
	})

	Context("When reconciling the underlay", func() {
		BeforeEach(func() {
			state = &ReconcileState{
				devices: map[string]*corev1alpha1.Device{
					"device-a": {Name: "device-a", UID: "device-a-uid"},
				},
				interfaces: map[string]*corev1alpha1.Interface{
					"eth0": newUnderlayInterface("eth0", "device-a", "10.0.0.1/30"),
				},
				loopbacks: map[string]*corev1alpha1.Interface{
					"device-a": {Name: "lo0", Namespace: ic.Namespace, Spec: corev1alpha1.InterfaceSpec{
						DeviceRef: corev1alpha1.LocalObjectReference{Name: "device-a"},
						Type:      corev1alpha1.InterfaceTypeLoopback,
					}},
				},
			}
		})

		DescribeTable("rejects unsupported protocols and missing dependencies", func(protocol nxoverlayv1alpha1.UnderlayProtocol, missing string, message string) {
			ic.Spec.DCI.Underlay.Protocol = protocol
			switch missing {
			case "devices":
				state.devices = nil
			case "interfaces":
				state.interfaces = nil
			case "loopbacks":
				state.loopbacks = nil
			case "device-loopback":
				state.loopbacks["device-b"] = state.loopbacks["device-a"]
				delete(state.loopbacks, "device-a")
			}
			_, err := r.reconcileUnderlay(ctx, ic, state)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
			Entry("BGP underlay", nxoverlayv1alpha1.UnderlayProtocolBGP, "", "BGP underlay is not supported"),
			Entry("no interfaces", nxoverlayv1alpha1.UnderlayProtocolOSPF, "interfaces", "no interfaces resolved"),
			Entry("no devices", nxoverlayv1alpha1.UnderlayProtocolOSPF, "devices", "no devices resolved"),
			Entry("no loopbacks", nxoverlayv1alpha1.UnderlayProtocolOSPF, "loopbacks", "no loopback interfaces resolved"),
			Entry("no loopback for the device", nxoverlayv1alpha1.UnderlayProtocolOSPF, "device-loopback", "no loopback for device device-a"),
		)

		DescribeTable("creates one OSPF resource per device", func(deviceCount, interfacesPerDevice int) {
			state.devices = make(map[string]*corev1alpha1.Device)
			state.interfaces = make(map[string]*corev1alpha1.Interface)
			state.loopbacks = make(map[string]*corev1alpha1.Interface)
			for deviceIndex := range deviceCount {
				deviceName := fmt.Sprintf("device-%d", deviceIndex)
				state.devices[deviceName] = &corev1alpha1.Device{Name: deviceName, UID: types.UID(deviceName)}
				state.loopbacks[deviceName] = &corev1alpha1.Interface{Name: deviceName + "-lo0", Namespace: ic.Namespace}
				for interfaceIndex := range interfacesPerDevice {
					name := fmt.Sprintf("%s-eth%d", deviceName, interfaceIndex)
					state.interfaces[name] = newUnderlayInterface(name, deviceName, fmt.Sprintf("10.0.0.%d/31", deviceIndex))
				}
			}
			_, err := r.reconcileUnderlay(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			ospfs := &corev1alpha1.OSPFList{}
			Expect(r.List(ctx, ospfs)).To(Succeed())
			Expect(ospfs.Items).To(HaveLen(deviceCount))
			for index := range ospfs.Items {
				ospf := &ospfs.Items[index]
				deviceName := ospf.Spec.DeviceRef.Name
				Expect(ospf.Name).To(Equal("underlay-" + hash(ic, state.devices[deviceName])))
				Expect(ospf.Spec.Instance).To(Equal("UNDERLAY"))
				Expect(ospf.Spec.RouterID).To(Equal(state.interfaces[deviceName+"-eth0"].Spec.IPv4.Addresses[0].Addr().String()))
				Expect(ospf.Labels).To(HaveKeyWithValue(nxoverlayv1alpha1.EVPNInterconnectLabel, ic.Name))
				Expect(ospf.OwnerReferences).To(ContainElement(SatisfyAll(
					HaveField("Kind", "EVPNInterconnect"),
					HaveField("Name", ic.Name),
					HaveField("UID", ic.UID),
				)))
				Expect(ospf.Spec.InterfaceRefs).To(HaveLen(interfacesPerDevice + 1))
				for interfaceIndex := range interfacesPerDevice {
					Expect(ospf.Spec.InterfaceRefs).To(ContainElement(HaveField("Name", fmt.Sprintf("%s-eth%d", deviceName, interfaceIndex))))
				}
				Expect(ospf.Spec.InterfaceRefs).To(ContainElement(SatisfyAll(
					HaveField("Name", state.loopbacks[deviceName].Name),
					HaveField("Passive", HaveValue(BeTrue())),
				)))
			}
		},
			Entry("one device", 1, 1),
			Entry("two devices", 2, 1),
			Entry("multiple interfaces on one device", 1, 2),
		)

		It("does not patch unchanged OSPF configuration", func() {
			_, err := r.reconcileUnderlay(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			patches := recorded.patches
			_, err = r.reconcileUnderlay(ctx, ic, state)
			Expect(err).NotTo(HaveOccurred())
			Expect(recorded.patches).To(Equal(patches))
		})
	})

	Context("When deriving the OSPF router ID", func() {
		DescribeTable("derives a consistent IPv4 router ID", func(addresses []string, expected, message string) {
			interfaces := make([]*corev1alpha1.Interface, len(addresses))
			for index, address := range addresses {
				name := fmt.Sprintf("eth%d", index)
				if address == "" {
					interfaces[index] = &corev1alpha1.Interface{Name: name}
				} else {
					interfaces[index] = newUnderlayInterface(name, "device-a", address)
				}
			}
			routerID, err := ospfRouterID(interfaces)
			if message != "" {
				Expect(err).To(MatchError(ContainSubstring(message)))
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(routerID).To(Equal(expected))
		},
			Entry("one interface", []string{"10.0.0.1/30"}, "10.0.0.1", ""),
			Entry("matching addresses", []string{"10.0.0.1/30", "10.0.0.1/30", "10.0.0.1/30"}, "10.0.0.1", ""),
			Entry("missing IPv4 configuration", []string{""}, "", "eth0"),
			Entry("different /30 addresses", []string{"10.0.0.1/30", "10.0.0.5/30"}, "", "eth1"),
			Entry("different /31 addresses", []string{"10.0.0.0/31", "10.0.0.2/31"}, "", "eth1"),
		)
	})
})

func testInterconnect() *nxoverlayv1alpha1.EVPNInterconnect {
	return &nxoverlayv1alpha1.EVPNInterconnect{
		Name: "interconnect", Namespace: "default", UID: "interconnect-uid",
		Spec: nxoverlayv1alpha1.EVPNInterconnectSpec{
			IPAddressPoolRef:        corev1alpha1.LocalObjectReference{Name: "loopbacks"},
			MultiSiteIDIndexPoolRef: corev1alpha1.LocalObjectReference{Name: "sites"},
			Interfaces: []nxoverlayv1alpha1.DCIInterface{
				{InterfaceRef: corev1alpha1.LocalObjectReference{Name: "dci-1"}, Address: corev1alpha1.MustParsePrefix("192.0.2.0/31")},
				{InterfaceRef: corev1alpha1.LocalObjectReference{Name: "dci-2"}, Address: corev1alpha1.MustParsePrefix("192.0.2.1/31")},
			},
			DCI: nxoverlayv1alpha1.DCI{Underlay: nxoverlayv1alpha1.Underlay{Protocol: nxoverlayv1alpha1.UnderlayProtocolOSPF}, Overlay: nxoverlayv1alpha1.Overlay{Protocol: nxoverlayv1alpha1.OverlayProtocolBGP}},
		},
	}
}

func allocateInterconnectClaim(c client.Client, name, value string) {
	claim := &poolv1alpha1.Claim{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, claim)).To(Succeed())
	Expect(claim.Spec.PoolRef.Kind).To(Equal("IPAddressPool"))
	Expect(claim.Spec.PoolRef.Name).To(Equal("loopbacks"))
	claim.Status.Value = value
	claim.Status.Conditions = []metav1.Condition{{Type: poolv1alpha1.AllocatedCondition, Status: metav1.ConditionTrue}}
	Expect(c.Status().Update(ctx, claim)).To(Succeed())
}

func completeInterconnectChildDeletion(c client.Client, obj client.Object) {
	Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	obj.SetFinalizers(nil)
	Expect(c.Update(ctx, obj)).To(Succeed())
}

type interconnectRecordingClient struct {
	client.Client
	patches     int
	deletes     []string
	deleteError error
}

func (c *interconnectRecordingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *interconnectRecordingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.deleteError != nil {
		return c.deleteError
	}
	c.deletes = append(c.deletes, obj.GetName())
	return c.Client.Delete(ctx, obj, opts...)
}

func prepareInterconnectInterfaces(r *EVPNInterconnectReconciler, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) {
	state.interfaces = make(map[string]*corev1alpha1.Interface)
	_, err := r.reconcileLoopbacks(ctx, ic, state)
	Expect(err).NotTo(HaveOccurred())
	for i := range ic.Spec.Interfaces {
		deviceName := fmt.Sprintf("device-%d", i+1)
		allocateInterconnectClaim(r.Client, "interconnect-"+deviceName+"-lo9", fmt.Sprintf("10.0.0.%d", i+1))
		intf := &corev1alpha1.Interface{
			Name: ic.Spec.Interfaces[i].InterfaceRef.Name, Namespace: ic.Namespace,
			Spec: corev1alpha1.InterfaceSpec{DeviceRef: corev1alpha1.LocalObjectReference{Name: deviceName}, Name: "Ethernet1/1", Type: corev1alpha1.InterfaceTypePhysical, AdminState: corev1alpha1.AdminStateUp},
		}
		Expect(r.Create(ctx, intf)).To(Succeed())
		state.interfaces[intf.Name] = intf
	}
	_, err = r.reconcileLoopbacks(ctx, ic, state)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("EVPNInterconnect Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			ic      *nxoverlayv1alpha1.EVPNInterconnect
			icKey   client.ObjectKey
			poolKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating an IndexPool for Multi-Site ID allocation")
			pool := &poolv1alpha1.IndexPool{
				GenerateName: "multisite-id-pool-",
				Namespace:    metav1.NamespaceDefault,
				Spec: poolv1alpha1.IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{{Start: 100, End: 199}},
				},
			}
			Expect(k8sClient.Create(ctx, pool)).To(Succeed())
			poolKey = client.ObjectKeyFromObject(pool)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
			})

			By("Creating the EVPNInterconnect resource")
			ic = &nxoverlayv1alpha1.EVPNInterconnect{
				GenerateName: "test-ic-",
				Namespace:    metav1.NamespaceDefault,
				Spec: nxoverlayv1alpha1.EVPNInterconnectSpec{
					MultiSiteIDIndexPoolRef: corev1alpha1.LocalObjectReference{
						Name: pool.Name,
					},
					IPAddressPoolRef: corev1alpha1.LocalObjectReference{Name: "unused-ip-pool"},
					Interfaces: []nxoverlayv1alpha1.DCIInterface{
						{
							InterfaceRef: corev1alpha1.LocalObjectReference{Name: "eth0"},
							Address:      corev1alpha1.MustParsePrefix("10.0.0.0/31"),
						},
						{
							InterfaceRef: corev1alpha1.LocalObjectReference{Name: "eth1"},
							Address:      corev1alpha1.MustParsePrefix("10.0.1.0/31"),
						},
					},
					DCI: nxoverlayv1alpha1.DCI{
						Underlay: nxoverlayv1alpha1.Underlay{Protocol: nxoverlayv1alpha1.UnderlayProtocolOSPF},
						Overlay: nxoverlayv1alpha1.Overlay{
							Protocol:                 nxoverlayv1alpha1.OverlayProtocolBGP,
							EVPNInterconnectSelector: metav1.LabelSelector{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ic)).To(Succeed())
			icKey = client.ObjectKeyFromObject(ic)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ic))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(client.IgnoreNotFound(k8sClient.Get(ctx, icKey, &nxoverlayv1alpha1.EVPNInterconnect{}))).To(Succeed())
				}).Should(Succeed())

				// envtest has no garbage collector; delete Claims before the pool disappears
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(icKey.Namespace))).To(Succeed())
				Eventually(func(g Gomega) {
					list := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, list, client.InNamespace(icKey.Namespace))).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())
			})
		})

		It("adds a finalizer to the EVPNInterconnect", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, icKey, ic)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())
		})

		It("creates a Claim owned by the EVPNInterconnect and allocates a Multi-Site ID from the pool", func() {
			By("Waiting for the EVPNInterconnect to be reconciled")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, icKey, ic)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			claimName := "multisiteid-" + hash(ic)

			By("Verifying the Claim is created with the correct pool reference and owner")
			claim := &poolv1alpha1.Claim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: claimName, Namespace: icKey.Namespace}, claim)).To(Succeed())
				g.Expect(claim.Spec.PoolRef).To(Equal(corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       poolKey.Name,
				}))
				g.Expect(claim.Labels).To(HaveKeyWithValue(nxoverlayv1alpha1.EVPNInterconnectLabel, icKey.Name))
				g.Expect(claim.OwnerReferences).To(ContainElement(SatisfyAll(
					HaveField("Kind", "EVPNInterconnect"),
					HaveField("Name", icKey.Name),
					HaveField("Controller", HaveValue(BeTrue())),
				)))
			}).Should(Succeed())

			By("Verifying the pool controller allocates an Index from the pool range")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), claim)).To(Succeed())
				g.Expect(claim.Status.Value).NotTo(BeEmpty())
				g.Expect(claim.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", poolv1alpha1.AllocatedCondition),
					HaveField("Status", metav1.ConditionTrue),
				)))
			}).Should(Succeed())

			Expect(claim.Status.AllocationRef).NotTo(BeNil())
			index := &poolv1alpha1.Index{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: icKey.Namespace, Name: claim.Status.AllocationRef.Name}, index)).To(Succeed())
			Expect(index.Spec.PoolRef.Name).To(Equal(poolKey.Name))
		})
	})
})

var _ = Describe("reconcileInterfaces", func() {
	var (
		r       *EVPNInterconnectReconciler
		created []client.Object
	)

	// newState returns a ReconcileState with all maps initialized, as the real
	// reconcile() does; the function nil-map panics otherwise.
	newState := func() *ReconcileState {
		return &ReconcileState{
			vteps:      make(map[string][]corev1alpha1.NetworkVirtualizationEdge),
			devices:    make(map[string]*corev1alpha1.Device),
			interfaces: make(map[string]*corev1alpha1.Interface),
		}
	}

	// createDevice creates a minimal valid Device and tracks it for cleanup.
	createDevice := func(name string) {
		device := &corev1alpha1.Device{
			Name: name, Namespace: metav1.NamespaceDefault,
			Spec: corev1alpha1.DeviceSpec{
				Endpoint: corev1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				Provider: "test-provider",
			},
		}
		Expect(k8sClient.Create(ctx, device)).To(Succeed())
		created = append(created, device)
	}

	// createSwitchportInterface creates a physical access-switchport interface on
	// the given device and tracks it for cleanup.
	createSwitchportInterface := func(name, deviceName string) {
		intf := &corev1alpha1.Interface{
			Name: name, Namespace: metav1.NamespaceDefault,
			Spec: corev1alpha1.InterfaceSpec{
				DeviceRef:  corev1alpha1.LocalObjectReference{Name: deviceName},
				Name:       "Ethernet1/1",
				Type:       corev1alpha1.InterfaceTypePhysical,
				AdminState: corev1alpha1.AdminStateUp,
				Switchport: &corev1alpha1.Switchport{Mode: corev1alpha1.SwitchportModeAccess, AccessVlan: 10},
			},
		}
		Expect(k8sClient.Create(ctx, intf)).To(Succeed())
		created = append(created, intf)
	}

	// dciInterface builds an in-memory DCIInterface referencing an interface by name.
	dciInterface := func(interfaceName, address string, mtu *int32) nxoverlayv1alpha1.DCIInterface {
		return nxoverlayv1alpha1.DCIInterface{
			InterfaceRef: corev1alpha1.LocalObjectReference{Name: interfaceName},
			Address:      corev1alpha1.MustParsePrefix(address),
			MTU:          mtu,
		}
	}

	// newIC builds an in-memory EVPNInterconnect; it is never persisted because
	// reconcileInterfaces only reads its Namespace, Name and Spec.Interfaces.
	newIC := func(interfaces ...nxoverlayv1alpha1.DCIInterface) *nxoverlayv1alpha1.EVPNInterconnect {
		return &nxoverlayv1alpha1.EVPNInterconnect{
			Name: "test-ic", Namespace: metav1.NamespaceDefault,
			Spec: nxoverlayv1alpha1.EVPNInterconnectSpec{Interfaces: interfaces},
		}
	}

	getInterface := func(name string) *corev1alpha1.Interface {
		intf := &corev1alpha1.Interface{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: name}, intf)).To(Succeed())
		return intf
	}

	BeforeEach(func() {
		r = &EVPNInterconnectReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		created = nil
	})

	AfterEach(func() {
		for _, obj := range created {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	})

	It("configures addressing and MTU on the happy path", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-b")

		mtu := int32(9000)
		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", &mtu),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)
		state := newState()

		res, err := r.reconcileInterfaces(ctx, ic, state)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		intfA := getInterface("intf-a")
		Expect(intfA.Spec.Switchport).To(BeNil())
		Expect(intfA.Spec.IPv4).NotTo(BeNil())
		Expect(intfA.Spec.IPv4.Addresses).To(HaveLen(1))
		Expect(intfA.Spec.IPv4.Addresses[0].String()).To(Equal("10.0.0.1/31"))
		Expect(intfA.Spec.MTU).To(Equal(int32(9000)))

		intfB := getInterface("intf-b")
		Expect(intfB.Spec.Switchport).To(BeNil())
		Expect(intfB.Spec.IPv4).NotTo(BeNil())
		Expect(intfB.Spec.IPv4.Addresses).To(HaveLen(1))
		Expect(intfB.Spec.IPv4.Addresses[0].String()).To(Equal("10.0.0.3/31"))
		Expect(intfB.Spec.MTU).To(Equal(int32(0)), "MTU must be untouched when dci.MTU is nil")

		Expect(state.devices).To(HaveLen(2))
		Expect(state.interfaces).To(HaveLen(2))
	})

	It("is idempotent and does not patch again when nothing changed", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-b")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).NotTo(HaveOccurred())

		rvA := getInterface("intf-a").ResourceVersion
		rvB := getInterface("intf-b").ResourceVersion

		_, err = r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).NotTo(HaveOccurred())

		Expect(getInterface("intf-a").ResourceVersion).To(Equal(rvA))
		Expect(getInterface("intf-b").ResourceVersion).To(Equal(rvB))
	})

	It("returns a terminal error when both interfaces are on the same device", func() {
		createDevice("dev-a")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly two distinct devices"))
	})

	It("returns a terminal error when an interface is referenced more than once", func() {
		createDevice("dev-a")
		createSwitchportInterface("intf-a", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-a", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("referenced more than once"))
	})

	It("waits without error when a referenced interface does not exist", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-missing", "10.0.0.3/31", nil),
		)
		state := newState()

		res, err := r.reconcileInterfaces(ctx, ic, state)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		intfA := getInterface("intf-a")
		Expect(intfA.Spec.Switchport).NotTo(BeNil(), "no interface should be patched when a reference is unresolved")
		Expect(intfA.Spec.IPv4).To(BeNil())
	})
})

// newUnderlayInterface builds an Interface pointer with one IPv4 address on the given device.
func newUnderlayInterface(name, deviceName, addr string) *corev1alpha1.Interface {
	return &corev1alpha1.Interface{
		Name:      name,
		Namespace: metav1.NamespaceDefault,
		Spec: corev1alpha1.InterfaceSpec{
			DeviceRef: corev1alpha1.LocalObjectReference{Name: deviceName},
			Type:      corev1alpha1.InterfaceTypePhysical,
			IPv4: &corev1alpha1.InterfaceIPv4{
				Addresses: []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix(addr)},
			},
		},
	}
}
