// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var _ = Describe("PIM Controller", func() {
	DescribeTable("Resolving local and remote RP members", func(members, localAddresses []string, wantLocal string, wantPeers []string, wantError bool) {
		pim := &v1alpha1.PIM{Spec: v1alpha1.PIMSpec{
			DeviceRef:        v1alpha1.LocalObjectReference{Name: "device"},
			RendezvousPoints: []v1alpha1.RendezvousPoint{{Address: "10.0.0.100", AnycastAddresses: slices.Clone(members)}},
		}}
		interfaces := make([]provider.PIMInterface, 0, len(localAddresses)+2)
		for i, addr := range localAddresses {
			iface := &v1alpha1.Interface{Name: fmt.Sprintf("lo%d", i), Spec: v1alpha1.InterfaceSpec{
				DeviceRef: pim.Spec.DeviceRef, Type: v1alpha1.InterfaceTypeLoopback,
			}}
			if addr != "" {
				iface.Spec.IPv4 = &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{
					v1alpha1.MustParsePrefix(addr + "/32"), v1alpha1.MustParsePrefix("10.0.0.9/32"),
				}}
			}
			interfaces = append(interfaces, provider.PIMInterface{Interface: iface})
		}
		// A matching address on an uplink or another Device does not identify the local RP.
		for _, device := range []string{"device", "other-device"} {
			iface := &v1alpha1.Interface{Name: device, Spec: v1alpha1.InterfaceSpec{
				DeviceRef: v1alpha1.LocalObjectReference{Name: device}, Type: v1alpha1.InterfaceTypePhysical,
				IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix("10.0.0.2/32")}},
			}}
			if device == "other-device" {
				iface.Spec.Type = v1alpha1.InterfaceTypeLoopback
			}
			interfaces = append(interfaces, provider.PIMInterface{Interface: iface})
		}
		resolved, err := resolvePIMRendezvousPoints(pim, interfaces)
		if wantError {
			Expect(err).To(MatchError(ContainSubstring("multiple local loopback interfaces")))
			Expect(resolved).To(BeNil())
			return
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(HaveLen(1))
		Expect(resolved[0].RendezvousPoint).To(BeIdenticalTo(&pim.Spec.RendezvousPoints[0]))
		if wantLocal == "" {
			Expect(resolved[0].LocalInterface).To(BeNil())
		} else {
			Expect(resolved[0].LocalInterface.Name).To(Equal(wantLocal))
		}
		Expect(resolved[0].PeerAddresses).To(ConsistOf(wantPeers))
		Expect(pim.Spec.RendezvousPoints[0].AnycastAddresses).To(Equal(members))
	},
		Entry("complete membership", []string{"10.0.0.2", "10.0.0.1"}, []string{"10.0.0.1"}, "lo0", []string{"10.0.0.2"}, false),
		Entry("single RP", []string{"10.0.0.1"}, []string{"10.0.0.1"}, "lo0", []string{}, false),
		Entry("static RP", []string(nil), []string{"10.0.0.1"}, "", []string{}, false),
		Entry("legacy remote members", []string{"10.0.0.2"}, []string{"10.0.0.1"}, "", []string{"10.0.0.2"}, false),
		Entry("missing IPv4 address", []string{"10.0.0.1", "10.0.0.2"}, []string{""}, "", []string{"10.0.0.1", "10.0.0.2"}, false),
		Entry("multiple local matches", []string{"10.0.0.1", "10.0.0.2"}, []string{"10.0.0.1", "10.0.0.2"}, "", []string{}, true),
		Entry("secondary address remains a peer", []string{"10.0.0.1", "10.0.0.9"}, []string{"10.0.0.1"}, "lo0", []string{"10.0.0.9"}, false),
	)

	It("Resolves the local Interface independently for each RP group", func() {
		pim := &v1alpha1.PIM{Spec: v1alpha1.PIMSpec{
			DeviceRef: v1alpha1.LocalObjectReference{Name: "device"},
			RendezvousPoints: []v1alpha1.RendezvousPoint{
				{Address: "10.0.0.100", AnycastAddresses: []string{"10.0.0.1", "10.0.0.2"}},
				{Address: "10.0.0.101", AnycastAddresses: []string{"10.0.0.3", "10.0.0.4"}},
			},
		}}
		interfaces := make([]provider.PIMInterface, 0, 2)
		for i, addr := range []string{"10.0.0.1/32", "10.0.0.3/32"} {
			interfaces = append(interfaces, provider.PIMInterface{Interface: &v1alpha1.Interface{
				Name: fmt.Sprintf("lo%d", i), Spec: v1alpha1.InterfaceSpec{
					DeviceRef: pim.Spec.DeviceRef, Type: v1alpha1.InterfaceTypeLoopback,
					IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix(addr)}},
				},
			}})
		}
		resolved, err := resolvePIMRendezvousPoints(pim, interfaces)
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved).To(HaveLen(2))
		Expect(resolved[0].LocalInterface).To(BeIdenticalTo(interfaces[0].Interface))
		Expect(resolved[0].PeerAddresses).To(Equal([]string{"10.0.0.2"}))
		Expect(resolved[1].LocalInterface).To(BeIdenticalTo(interfaces[1].Interface))
		Expect(resolved[1].PeerAddresses).To(Equal([]string{"10.0.0.4"}))
	})

	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				GenerateName: "test-pim-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind PIM")
			resource := &v1alpha1.PIM{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.PIMSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the PIM resource")
			pim := &v1alpha1.PIM{}
			pim.Name = name
			pim.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pim))).To(Succeed())

			By("Verifying the resource is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(name).PIM).To(BeNil(), "Provider should not have PIM instance configured")
			}).Should(Succeed())

			By("Waiting for the PIM to be fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.PIM{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleanup the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.PIM{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.PIM{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.PIM{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.PIM{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(name).PIM).ToNot(BeNil(), "Provider should have PIM instance configured")
			}).Should(Succeed())
		})
	})

	Context("When an interfaceRef does not exist", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				GenerateName: "test-pim-missing-intf-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the PIM resource")
			pim := &v1alpha1.PIM{}
			pim.Name = name
			pim.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pim))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.PIM{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleanup the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should set ConfiguredCondition to false when interfaceRef does not exist", func() {
			By("Creating a PIM resource with a non-existent interfaceRef")
			pim := &v1alpha1.PIM{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.PIMSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					InterfaceRefs: []v1alpha1.PIMInterface{
						{
							Name: "non-existing-interface",
							Mode: v1alpha1.PIMModeSparse,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pim)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to false with InterfaceNotFound reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.PIM{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())

				ready := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))

				configured := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(configured).NotTo(BeNil())
				g.Expect(configured.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(configured.Reason).To(Equal(v1alpha1.InterfaceNotFoundReason))
			}).Should(Succeed())
		})
	})
})
