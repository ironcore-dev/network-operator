// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("DHCPRelay Controller with deprecated API fields", func() {
	It("Should reconcile interfaceRefs", func() {
		device := &v1alpha1.Device{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-deprecated-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "192.168.20.50:9339"}}}
		Expect(k8sClient.Create(ctx, device)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		vlan := &v1alpha1.VLAN{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-deprecated-vlan-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.VLANSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name}, ID: 70, Name: "vlan70"}}
		Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vlan))).To(Succeed())
		})

		intf := &v1alpha1.Interface{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-deprecated-intf-", Namespace: metav1.NamespaceDefault},
			Spec: v1alpha1.InterfaceSpec{
				DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name}, Name: "vlan70", Type: v1alpha1.InterfaceTypeRoutedVLAN, AdminState: v1alpha1.AdminStateUp,
				VlanRef: &v1alpha1.LocalObjectReference{Name: vlan.Name}, IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.7.1/24")}}},
			},
		}
		Expect(k8sClient.Create(ctx, intf)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			configuredInterface := &v1alpha1.Interface{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(intf), configuredInterface)).To(Succeed())
			condition := meta.FindStatusCondition(configuredInterface.Status.Conditions, v1alpha1.ConfiguredCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())

		relay := &v1alpha1.DHCPRelay{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-deprecated-", Namespace: metav1.NamespaceDefault},
			Spec:       v1alpha1.DHCPRelaySpec{DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name}, InterfaceRefs: []v1alpha1.LocalObjectReference{{Name: intf.Name}}, Servers: []string{"192.168.1.1"}},
		}
		Expect(k8sClient.Create(ctx, relay)).To(Succeed())
		relayKey := client.ObjectKeyFromObject(relay)
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, relay))).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, relayKey, &v1alpha1.DHCPRelay{}))).To(BeTrue())
				g.Expect(testProvider.DHCPRelay).To(BeNil())
			}).Should(Succeed())
		})

		Eventually(func(g Gomega) {
			g.Expect(testProvider.DHCPRelay).ToNot(BeNil())
			g.Expect(testProvider.DHCPRelay.GetName()).To(Equal(relay.Name))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			configuredRelay := &v1alpha1.DHCPRelay{}
			g.Expect(k8sClient.Get(ctx, relayKey, configuredRelay)).To(Succeed())
			condition := meta.FindStatusCondition(configuredRelay.Status.Conditions, v1alpha1.ReadyCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})
})
