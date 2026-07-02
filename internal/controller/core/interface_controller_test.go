// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("Interface Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-interface-",
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
		})

		AfterEach(func() {
			By("Cleaning up all Interface resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Interface{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleaning up test VLAN resource")
			vlan := &v1alpha1.VLAN{}
			if err := k8sClient.Get(ctx, key, vlan); err == nil {
				Expect(k8sClient.Delete(ctx, vlan)).To(Succeed())
			}

			By("Cleaning up test VRF resource")
			vrf := &v1alpha1.VRF{}
			if err := k8sClient.Get(ctx, key, vrf); err == nil {
				Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())
			}

			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying all Interfaces are deleted")
			Eventually(func(g Gomega) {
				intfList := &v1alpha1.InterfaceList{}
				g.Expect(k8sClient.List(ctx, intfList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				g.Expect(intfList.Items).To(BeEmpty())
			}).Should(Succeed())

			By("Verifying the Interface is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeFalse(), "Provider shouldn't have Interface configured anymore")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a Physical Interface with IPv4 addresses", func() {
			By("Creating an Interface with IPv4 addresses")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

			By("Verifying the Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have Interface configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a Physical Interface with unnumbered IPv4", func() {
			By("Creating a Loopback Interface with IPv4 addresses")
			lb := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-lb",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, lb)).To(Succeed())

			By("Creating a Physical Interface with unnumbered IPv4 configuration")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: lb.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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
		})

		It("Should handle unnumbered reference to Interface from different device", func() {
			By("Creating a Loopback Interface on a different device")
			lb := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-lb",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "non-existing-device"},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
				},
			}
			Expect(k8sClient.Create(ctx, lb)).To(Succeed())

			By("Creating a Physical Interface with unnumbered reference to the cross-device Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: lb.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets invalid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

		It("Should handle unnumbered reference to non-existing Interface", func() {
			By("Creating a Physical Interface with unnumbered reference to non-existing Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: "non-existing-interface"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets Interface not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

		const memberName1 = "test-member-1"
		const memberName2 = "test-member-2"

		It("Should successfully reconcile an Aggregate Interface with valid member interfaces", func() {
			By("Creating Physical member interfaces")
			member1 := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName1,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       memberName1,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member1)).To(Succeed())

			member2 := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName2,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       memberName2,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member2)).To(Succeed())

			By("Creating an Aggregate Interface")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test Aggregate Interface",
					Type:        v1alpha1.InterfaceTypeAggregate,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: memberName1},
							{Name: memberName2},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

			By("Verifying member interfaces are properly linked")
			Eventually(func(g Gomega) {
				memberIntf1 := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: memberName1, Namespace: metav1.NamespaceDefault}, memberIntf1)).To(Succeed())
				g.Expect(memberIntf1.Status.MemberOf).ToNot(BeNil())
				g.Expect(memberIntf1.Status.MemberOf.Name).To(Equal(name))
				g.Expect(memberIntf1.Labels).To(HaveKeyWithValue(v1alpha1.AggregateLabel, name))

				memberIntf2 := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: memberName2, Namespace: metav1.NamespaceDefault}, memberIntf2)).To(Succeed())
				g.Expect(memberIntf2.Status.MemberOf).ToNot(BeNil())
				g.Expect(memberIntf2.Status.MemberOf.Name).To(Equal(name))
				g.Expect(memberIntf2.Labels).To(HaveKeyWithValue(v1alpha1.AggregateLabel, name))
			}).Should(Succeed())

			By("Verifying the Aggregate Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have Aggregate Interface configured")
			}).Should(Succeed())
		})

		It("Should handle member interface not found", func() {
			By("Creating an Aggregate Interface with non-existing member")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeAggregate,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: "non-existing-member"},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets interface not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

		It("Should handle member interface on different device", func() {
			By("Creating a member interface on different device")
			member := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName1,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:       memberName1,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())

			By("Creating an Aggregate Interface referencing the cross-device member")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeAggregate,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: memberName1},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets cross-device reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

		It("Should handle member interface already in use by another aggregate", func() {
			By("Creating a Physical member interface")
			member := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName1,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       memberName1,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())

			By("Setting the member interface status to indicate it's already in use")
			orig := member.DeepCopy()
			member.Status.MemberOf = &v1alpha1.LocalObjectReference{Name: "existing-aggregate"}
			Expect(k8sClient.Status().Patch(ctx, member, client.MergeFrom(orig))).To(Succeed())

			By("Creating an Aggregate Interface referencing the already-in-use member")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeAggregate,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: memberName1},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets member already in use status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.MemberInterfaceAlreadyInUseReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle member interface with invalid type", func() {
			By("Creating a Loopback interface (invalid type for aggregation)")
			member := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName1,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       memberName1,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback, // Invalid type for member interface
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())

			By("Creating an Aggregate Interface referencing the invalid-type member")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeAggregate,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: memberName1},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets invalid type status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile an Aggregate Interface with IPv4 addresses and VRF", func() {
			By("Creating a VRF resource")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "PROD",
					VNI:       1000,
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())

			By("Creating a Physical member interface")
			member := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      memberName1,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       memberName1,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())

			By("Creating an L3 Aggregate Interface with IPv4 and VRF")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test L3 Aggregate Interface",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypeAggregate,
					VrfRef:      &v1alpha1.LocalObjectReference{Name: vrf.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/31")}},
					},
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: memberName1},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

			By("Verifying the Interface has the VRF label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.VRFLabel, vrf.Name))
			}).Should(Succeed())

			By("Verifying member interface is properly linked")
			Eventually(func(g Gomega) {
				memberIntf := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: memberName1, Namespace: metav1.NamespaceDefault}, memberIntf)).To(Succeed())
				g.Expect(memberIntf.Status.MemberOf).ToNot(BeNil())
				g.Expect(memberIntf.Status.MemberOf.Name).To(Equal(name))
				g.Expect(memberIntf.Labels).To(HaveKeyWithValue(v1alpha1.AggregateLabel, name))
			}).Should(Succeed())

			By("Verifying the Aggregate Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have L3 Aggregate Interface configured")
			}).Should(Succeed())
		})

		It("Should reconcile a Physical interface that is a member of an L3 Aggregate", func() {
			By("Creating an L3 Aggregate interface")
			aggregate := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "po100",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "po100",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeAggregate,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.100.0/31")}},
					},
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{
							{Name: "eth1-100"},
						},
						ControlProtocol: v1alpha1.ControlProtocol{
							Mode: v1alpha1.LACPModeActive,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aggregate)).To(Succeed())

			By("Creating the member Physical interface")
			member := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eth1-100",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "eth1-100",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, member)).To(Succeed())

			By("Verifying the member eventually gets MemberOf set by the Aggregate reconciliation")
			Eventually(func(g Gomega) {
				m := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "eth1-100", Namespace: metav1.NamespaceDefault}, m)).To(Succeed())
				g.Expect(m.Status.MemberOf).NotTo(BeNil())
				g.Expect(m.Status.MemberOf.Name).To(Equal("po100"))
			}).Should(Succeed())

			By("Verifying the member Physical interface reconciles without error")
			Eventually(func(g Gomega) {
				m := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "eth1-100", Namespace: metav1.NamespaceDefault}, m)).To(Succeed())
				g.Expect(m.Labels).To(HaveKeyWithValue(v1alpha1.AggregateLabel, "po100"))
			}).Should(Succeed())

			By("Verifying the member Physical interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has("eth1-100")).To(BeTrue(), "Provider should have member Physical Interface configured")
			}).Should(Succeed())
		})

		It("Should fail reconcile when parent interface does not exist for subinterface", func() {
			By("Creating a Subinterface referencing a non-existent parent")
			subintf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name + ".100",
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Subinterface without parent",
					Type:        v1alpha1.InterfaceTypeSubinterface,
					ParentInterfaceRef: &v1alpha1.LocalObjectReference{
						Name: "non-existent-parent",
					},
					Encapsulation: &v1alpha1.Encapsulation{
						Type: v1alpha1.EncapsulationTypeDot1Q,
						Tag:  100,
					},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.100/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, subintf)).To(Succeed())

			By("Verifying the controller sets parent interface not ready status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.ParentInterfaceNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile parent Physical interface and Subinterface", func() {
			const parentName = "test-parent-eth"
			const subinterfaceName = "test-subintf"

			By("Creating a Physical parent interface")
			parentIntf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      parentName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        parentName,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Parent Physical Interface",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, parentIntf)).To(Succeed())

			By("Verifying the parent Physical interface is ready")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: parentName, Namespace: metav1.NamespaceDefault}, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating a Subinterface referencing the parent")
			subintf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subinterfaceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        parentName + ".100",
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Subinterface with 802.1q encapsulation",
					Type:        v1alpha1.InterfaceTypeSubinterface,
					ParentInterfaceRef: &v1alpha1.LocalObjectReference{
						Name: parentName,
					},
					Encapsulation: &v1alpha1.Encapsulation{
						Type: v1alpha1.EncapsulationTypeDot1Q,
						Tag:  100,
					},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.100.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, subintf)).To(Succeed())

			By("Verifying the controller sets the parent interface as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: subinterfaceName, Namespace: metav1.NamespaceDefault}, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Interface"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(parentName))
			}).Should(Succeed())

			By("Verifying the subinterface status is ready")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: subinterfaceName, Namespace: metav1.NamespaceDefault}, resource)).To(Succeed())
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

			By("Verifying the Subinterface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(parentName+".100")).To(BeTrue(), "Provider should have Subinterface configured")
			}).Should(Succeed())

			By("Verifying the parent Physical interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(parentName)).To(BeTrue(), "Provider should have parent Physical Interface configured")
			}).Should(Succeed())
		})

		It("Should handle unnumbered reference to non-Loopback Interface", func() {
			By("Creating a Physical Interface to be referenced")
			phys := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-phys",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name + "-phys",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.2/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, phys)).To(Succeed())

			By("Creating a Physical Interface with unnumbered reference to the Physical Interface")
			eth := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: phys.Name},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, eth)).To(Succeed())

			By("Verifying the controller sets invalid interface type status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutedVLAN Interface with IPv4 addresses", func() {
			By("Creating a VLAN resource")
			vlan := &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					ID:         100,
					Name:       "test-vlan",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())

			By("Creating a RoutedVLAN Interface with IPv4 addresses")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test RoutedVLAN Interface",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:     &v1alpha1.LocalObjectReference{Name: vlan.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("192.168.100.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

			By("Verifying the VLAN status is updated with RoutedBy reference")
			Eventually(func(g Gomega) {
				vlanResource := &v1alpha1.VLAN{}
				g.Expect(k8sClient.Get(ctx, key, vlanResource)).To(Succeed())
				g.Expect(vlanResource.Status.RoutedBy).ToNot(BeNil())
				g.Expect(vlanResource.Status.RoutedBy.Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have RoutedVLAN Interface configured")
			}).Should(Succeed())
		})

		It("Should handle RoutedVLAN Interface referencing non-existent VLAN", func() {
			By("Creating a RoutedVLAN Interface referencing a non-existent VLAN")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: "non-existent-vlan"},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("192.168.100.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller sets VLAN not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.VLANNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle RoutedVLAN Interface referencing VLAN on different device", func() {
			By("Creating a VLAN on a different device")
			vlan := &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					ID:         100,
					Name:       "test-vlan",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())

			By("Creating a RoutedVLAN Interface referencing the cross-device VLAN")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: vlan.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("192.168.100.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller sets cross-device reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

		It("Should successfully reconcile an Interface with VRF reference", func() {
			By("Creating a VRF resource")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "test-vrf",
					VNI:       1000,
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())

			By("Creating a Loopback Interface with VRF reference")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: name},
					Name:        name,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test Interface with VRF",
					Type:        v1alpha1.InterfaceTypeLoopback,
					VrfRef:      &v1alpha1.LocalObjectReference{Name: vrf.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.1.1.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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

			By("Verifying the Interface has the VRF label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.VRFLabel, vrf.Name))
			}).Should(Succeed())

			By("Verifying the Interface is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Ports.Has(name)).To(BeTrue(), "Provider should have Interface with VRF configured")
			}).Should(Succeed())
		})

		It("Should handle Interface referencing non-existent VRF", func() {
			By("Creating an Interface referencing a non-existent VRF")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
					VrfRef:     &v1alpha1.LocalObjectReference{Name: "non-existent-vrf"},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.1.1.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller sets VRF not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.VRFNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle Interface referencing VRF on different device", func() {
			By("Creating a VRF on a different device")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:      "test-vrf",
					VNI:       1000,
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())

			By("Creating an Interface referencing the cross-device VRF")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name,
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeLoopback,
					VrfRef:     &v1alpha1.LocalObjectReference{Name: vrf.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.1.1.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Verifying the controller sets cross-device reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
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
	})

	Context("When DNS domain changes on a neighboring device", func() {
		var (
			localDevice  *v1alpha1.Device
			remoteDevice *v1alpha1.Device
			localIntf    *v1alpha1.Interface
			remoteIntf   *v1alpha1.Interface
			dns          *v1alpha1.DNS
		)

		BeforeEach(func() {
			By("Creating local and remote Device resources")
			localDevice = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "local-device-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.10:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, localDevice)).To(Succeed())

			remoteDevice = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "remote-device-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.11:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, remoteDevice)).To(Succeed())

			By("Waiting for the device controller to set hostname on remote device and patching it")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(remoteDevice), remoteDevice)).To(Succeed())
				g.Expect(remoteDevice.Status.LastRebootTime.IsZero()).To(BeFalse())
			}).Should(Succeed())
			remoteDevice.Status.Hostname = "remote-switch"
			Expect(k8sClient.Status().Update(ctx, remoteDevice)).To(Succeed())

			By("Creating a remote Interface on the remote device")
			remoteIntf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "remote-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: remoteDevice.Name},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, remoteIntf)).To(Succeed())

			By("Creating a DNS resource for the remote device")
			dns = &v1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "remote-dns-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DNSSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: remoteDevice.Name},
					Domain:    "example.com",
				},
			}
			Expect(k8sClient.Create(ctx, dns)).To(Succeed())

			By("Waiting for the local device controller to initialize")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(localDevice), localDevice)).To(Succeed())
				g.Expect(localDevice.Status.LastRebootTime.IsZero()).To(BeFalse())
			}).Should(Succeed())

			By("Configuring LLDP neighbor on the provider for the local interface")
			testProvider.SetLLDPNeighbor("Ethernet1/2", "remote-switch.example.com", "aa:bb:cc:dd:ee:ff", "Ethernet1/1", 120)

			By("Creating a local Physical Interface with neighbor label pointing to the remote interface")
			localIntf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "local-intf-",
					Namespace:    metav1.NamespaceDefault,
					Labels: map[string]string{
						v1alpha1.PhysicalInterfaceNeighborLabel: remoteIntf.Name,
					},
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: localDevice.Name},
					Name:       "Ethernet1/2",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, localIntf)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up LLDP neighbor configuration")
			testProvider.Lock()
			delete(testProvider.LLDPNeighbors, "Ethernet1/2")
			testProvider.Unlock()

			By("Cleaning up all Interface resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.Interface{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
			Eventually(func(g Gomega) {
				intfList := &v1alpha1.InterfaceList{}
				g.Expect(k8sClient.List(ctx, intfList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
				g.Expect(intfList.Items).To(BeEmpty())
			}).Should(Succeed())

			By("Cleaning up DNS resource")
			if dns != nil {
				Expect(k8sClient.Delete(ctx, dns)).To(Succeed())
			}

			By("Cleaning up Device resources")
			if localDevice != nil {
				Expect(k8sClient.Delete(ctx, localDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
			if remoteDevice != nil {
				Expect(k8sClient.Delete(ctx, remoteDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
		})

		It("Should verify LLDP neighbor and re-validate when DNS domain changes", func() {
			By("Verifying the neighbor validation is Verified")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(localIntf), resource)).To(Succeed())
				g.Expect(resource.Status.Neighbors).To(HaveLen(1))
				g.Expect(resource.Status.Neighbors[0].Validation).To(Equal(v1alpha1.NeighborVerified))
			}).Should(Succeed())

			By("Updating the DNS domain on the remote device")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dns), dns)).To(Succeed())
				dns.Spec.Domain = "new.example.com"
				g.Expect(k8sClient.Update(ctx, dns)).To(Succeed())
			}).Should(Succeed())

			By("Verifying the neighbor validation changes to DeviceMismatch")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(localIntf), resource)).To(Succeed())
				g.Expect(resource.Status.Neighbors).To(HaveLen(1))
				g.Expect(resource.Status.Neighbors[0].Validation).To(Equal(v1alpha1.NeighborDeviceMismatch))
			}).Should(Succeed())
		})
	})

	Context("Interface Update Predicate", func() {
		var p interfaceUpdatePredicate

		BeforeEach(func() {
			p = interfaceUpdatePredicate{}
		})

		It("Should allow update when Neighbors field is added", func() {
			oldIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Neighbors: nil,
				},
			}
			newIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Neighbors: []v1alpha1.Neighbor{{
						SystemName: "switch-1",
						ChassisID:  "00:11:22:33:44:55",
						PortID:     "Eth1/1",
					}},
				},
			}
			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeTrue())
		})

		It("Should allow update when neighbor SystemName changes", func() {
			oldIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Neighbors: []v1alpha1.Neighbor{{
						SystemName:     "switch-1",
						ChassisID:      "00:11:22:33:44:55",
						PortID:         "Eth1/1",
						ExpirationTime: metav1.NewTime(time.Now()),
					}},
				},
			}
			newIntf := oldIntf.DeepCopy()
			newIntf.Status.Neighbors[0].SystemName = "switch-2"

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeTrue())
		})

		It("Should block update when only ExpirationTime changes", func() {
			now := time.Now()
			conditions := []metav1.Condition{
				{Type: v1alpha1.ReadyCondition, Status: metav1.ConditionTrue},
				{Type: v1alpha1.ConfiguredCondition, Status: metav1.ConditionTrue},
				{Type: v1alpha1.OperationalCondition, Status: metav1.ConditionTrue},
				{Type: v1alpha1.PausedCondition, Status: metav1.ConditionFalse},
			}
			oldIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Conditions: conditions,
					Neighbors: []v1alpha1.Neighbor{{
						SystemName:     "switch-1",
						ChassisID:      "00:11:22:33:44:55",
						PortID:         "Eth1/1",
						ExpirationTime: metav1.NewTime(now),
					}},
				},
			}
			newIntf := oldIntf.DeepCopy()
			newIntf.Status.Neighbors[0].ExpirationTime = metav1.NewTime(now.Add(120 * time.Second))

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeFalse())
		})

		It("Should allow update when neighbor is removed", func() {
			oldIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Neighbors: []v1alpha1.Neighbor{{SystemName: "switch-1"}},
				},
			}
			newIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Neighbors: nil,
				},
			}
			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeTrue())
		})

		It("Should not rely on this predicate for generation changes", func() {
			oldIntf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.InterfaceStatus{
					Neighbors: []v1alpha1.Neighbor{{
						ExpirationTime: metav1.NewTime(time.Now()),
					}},
				},
			}
			newIntf := oldIntf.DeepCopy()
			newIntf.Generation = 2
			newIntf.Status.Neighbors[0].ExpirationTime = metav1.NewTime(time.Now().Add(120 * time.Second))

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeFalse())
		})

		It("Should block no-op update when conditions are not fully initialized", func() {
			oldIntf := &v1alpha1.Interface{
				Status: v1alpha1.InterfaceStatus{
					Conditions: []metav1.Condition{
						{Type: v1alpha1.ReadyCondition, Status: metav1.ConditionUnknown},
						{Type: v1alpha1.PausedCondition, Status: metav1.ConditionTrue},
					},
				},
			}
			newIntf := oldIntf.DeepCopy()

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeFalse())
		})

		It("Should allow update when finalizers change", func() {
			oldIntf := &v1alpha1.Interface{}
			newIntf := oldIntf.DeepCopy()
			newIntf.Finalizers = []string{v1alpha1.FinalizerName}

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeTrue())
		})

		It("Should allow update when deletion timestamp changes", func() {
			oldIntf := &v1alpha1.Interface{}
			newIntf := oldIntf.DeepCopy()
			now := metav1.NewTime(time.Now())
			newIntf.DeletionTimestamp = &now

			e := event.UpdateEvent{ObjectOld: oldIntf, ObjectNew: newIntf}
			Expect(p.Update(e)).To(BeTrue())
		})
	})
})
