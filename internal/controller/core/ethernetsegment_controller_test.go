// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("EthernetSegment Controller", func() {
	Context("When reconciling a resource", func() {
		const esi = "00:11:22:33:44:55:66:77:88:01"
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-es-",
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
			By("Cleaning up all EthernetSegment resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EthernetSegment{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleaning up test Interface resource")
			intf := &v1alpha1.Interface{}
			if err := k8sClient.Get(ctx, key, intf); err == nil {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
			}

			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the EthernetSegment is removed from the provider")
			Eventually(func(g Gomega) {
				_, exists := testProvider.GetEthernetSegment(name)
				g.Expect(exists).To(BeFalse(), "Provider shouldn't have ESI configured anymore")
			}).Should(Succeed())
		})

		It("Should successfully reconcile an EthernetSegment", func() {
			By("Creating an Aggregate Interface with switchport config")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "port-channel10",
					Type:       v1alpha1.InterfaceTypeAggregate,
					AdminState: v1alpha1.AdminStateUp,
					Switchport: &v1alpha1.Switchport{
						Mode: v1alpha1.SwitchportModeTrunk,
					},
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{{Name: "eth1"}},
						ControlProtocol:     v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating an EthernetSegment")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: name},
					ESIType:        v1alpha1.ESITypeArbitrary,
					ESI:            esi,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
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

			By("Verifying the EthernetSegment is configured in the provider")
			Eventually(func(g Gomega) {
				storedESI, exists := testProvider.GetEthernetSegment(name)
				g.Expect(exists).To(BeTrue(), "Provider should have ESI configured")
				g.Expect(storedESI).To(Equal(esi))
			}).Should(Succeed())

			By("Verifying status ESI and ESIType are populated")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.ESI).To(Equal(esi))
				g.Expect(resource.Status.ESIType).To(Equal(v1alpha1.ESITypeArbitrary))
			}).Should(Succeed())
		})

		It("Should handle EthernetSegment referencing non-existent Interface", func() {
			By("Creating an EthernetSegment referencing a non-existent Interface")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: "non-existent-intf"},
					ESIType:        v1alpha1.ESITypeArbitrary,
					ESI:            esi,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the controller sets InterfaceNotFound status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InterfaceNotFoundReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle EthernetSegment referencing Interface on different device", func() {
			By("Creating an Interface on a different device")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:       "port-channel10",
					Type:       v1alpha1.InterfaceTypeAggregate,
					AdminState: v1alpha1.AdminStateUp,
					Switchport: &v1alpha1.Switchport{
						Mode: v1alpha1.SwitchportModeTrunk,
					},
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{{Name: "eth1"}},
						ControlProtocol:     v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating an EthernetSegment referencing the cross-device Interface")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: name},
					ESIType:        v1alpha1.ESITypeArbitrary,
					ESI:            esi,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the controller sets CrossDeviceReference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle EthernetSegment referencing non-Aggregate Interface", func() {
			By("Creating an Ethernet Interface (not Aggregate)")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "eth1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
					Switchport: &v1alpha1.Switchport{
						Mode: v1alpha1.SwitchportModeTrunk,
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating an EthernetSegment referencing the non-Aggregate Interface")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: name},
					ESIType:        v1alpha1.ESITypeArbitrary,
					ESI:            esi,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the controller sets InvalidInterfaceType status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle EthernetSegment referencing Interface without switchport", func() {
			By("Creating an Aggregate Interface without switchport config")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "port-channel10",
					Type:       v1alpha1.InterfaceTypeAggregate,
					AdminState: v1alpha1.AdminStateUp,
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{{Name: "eth1"}},
						ControlProtocol:     v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating an EthernetSegment referencing the Interface")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: name},
					ESIType:        v1alpha1.ESITypeArbitrary,
					ESI:            esi,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the controller sets InterfaceNotSwitchport status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(v1alpha1.InterfaceNotSwitchportReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})
		It("Should auto-derive ESI when ESIType is MAC and ESI is omitted", func() {
			By("Creating an Aggregate Interface with switchport config")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "port-channel20",
					Type:       v1alpha1.InterfaceTypeAggregate,
					AdminState: v1alpha1.AdminStateUp,
					Switchport: &v1alpha1.Switchport{
						Mode: v1alpha1.SwitchportModeTrunk,
					},
					Aggregation: &v1alpha1.Aggregation{
						MemberInterfaceRefs: []v1alpha1.LocalObjectReference{{Name: "eth1"}},
						ControlProtocol:     v1alpha1.ControlProtocol{Mode: v1alpha1.LACPModeActive},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating an EthernetSegment with ESIType MAC and no explicit ESI")
			es := &v1alpha1.EthernetSegment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EthernetSegmentSpec{
					DeviceRef:      v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:   v1alpha1.LocalObjectReference{Name: name},
					ESIType:        v1alpha1.ESITypeMAC,
					RedundancyMode: v1alpha1.RedundancyModeAllActive,
				},
			}
			Expect(k8sClient.Create(ctx, es)).To(Succeed())

			By("Verifying the status ESI is auto-populated from the provider")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EthernetSegment{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.ESI).To(Equal("03:aa:bb:cc:dd:ee:ff:00:00:01"))
				g.Expect(resource.Status.ESIType).To(Equal(v1alpha1.ESITypeMAC))
			}).Should(Succeed())
		})
	})
})
