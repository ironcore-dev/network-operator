// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("ISIS Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				GenerateName: "test-isis-",
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

			By("Creating the custom resource for the Kind ISIS")
			resource := &v1alpha1.ISIS{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.ISISSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					Instance:           "UNDERLAY",
					NetworkEntityTitle: "49.0001.0000.0000.0001.00",
					Type:               v1alpha1.ISISLevel1,
					OverloadBit:        v1alpha1.OverloadBitOnStartup,
					AddressFamilies: []v1alpha1.AddressFamily{
						v1alpha1.AddressFamilyIPv4Unicast,
						v1alpha1.AddressFamilyIPv6Unicast,
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the ISIS resource")
			isis := &v1alpha1.ISIS{}
			isis.Name = name
			isis.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, isis))).To(Succeed())

			By("Verifying the resource is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(name).ISIS.Has("UNDERLAY")).To(BeFalse(), "Provider should not have ISIS instance configured")
			}).Should(Succeed())

			By("Waiting for the ISIS to be fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.ISIS{})
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
				resource := &v1alpha1.ISIS{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ISIS{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ISIS{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ISIS{}
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
				g.Expect(testDevices.StateFor(name).ISIS.Has("UNDERLAY")).To(BeTrue(), "Provider should have ISIS instance configured")
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
				GenerateName: "test-isis-missing-intf-",
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
			By("Cleaning up the ISIS resource")
			isis := &v1alpha1.ISIS{}
			isis.Name = name
			isis.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, isis))).To(Succeed())

			By("Waiting for the ISIS to be fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.ISIS{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleanup the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should set ConfiguredCondition to false when interfaceRef does not exist", func() {
			By("Creating an ISIS resource with a non-existent interfaceRef")
			isis := &v1alpha1.ISIS{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.ISISSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					Instance:           "UNDERLAY",
					NetworkEntityTitle: "49.0001.0000.0000.0001.00",
					Type:               v1alpha1.ISISLevel1,
					AddressFamilies: []v1alpha1.AddressFamily{
						v1alpha1.AddressFamilyIPv4Unicast,
					},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: "non-existing-interface"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, isis)).To(Succeed())

			By("Verifying the controller sets ConfiguredCondition to false with InterfaceNotFound reason")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ISIS{}
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

	Context("When a referenced Interface is not configured", func() {
		var deviceName string

		BeforeEach(func() {
			By("Creating the Device resource")
			device := &v1alpha1.Device{
				GenerateName: "test-isis-intfnr-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
			})
		})

		It("Should set ConfiguredCondition to False with WaitingForDependenciesReason", func() {
			By("Creating an Interface referencing a non-existent VRF so it stays not configured")
			intf := &v1alpha1.Interface{
				GenerateName: "test-isis-intfnr-intf-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "Ethernet1/1",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypePhysical,
					VrfRef:     &v1alpha1.LocalObjectReference{Name: "test-isis-intfnr-nonexistent-vrf"},
					IPv4:       &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.5.1/24")}}},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			intfKey := client.ObjectKeyFromObject(intf)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, intfKey, &v1alpha1.Interface{}))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Verifying the Interface is not configured")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, intfKey, intf)).To(Succeed())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Creating an ISIS referencing the non-configured Interface")
			isis := &v1alpha1.ISIS{
				GenerateName: "test-isis-intfnr-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.ISISSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
					Instance:           "UNDERLAY",
					NetworkEntityTitle: "49.0001.0000.0000.0001.00",
					Type:               v1alpha1.ISISLevel1,
					AddressFamilies:    []v1alpha1.AddressFamily{v1alpha1.AddressFamilyIPv4Unicast},
					InterfaceRefs:      []v1alpha1.LocalObjectReference{{Name: intf.Name}},
				},
			}
			Expect(k8sClient.Create(ctx, isis)).To(Succeed())
			isisKey := client.ObjectKeyFromObject(isis)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, isis))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, isisKey, &v1alpha1.ISIS{}))).To(BeTrue())
				}).Should(Succeed())
			})

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, isisKey, isis)).To(Succeed())
				cond := meta.FindStatusCondition(isis.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))

				ready := meta.FindStatusCondition(isis.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(ready).ToNot(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the ISIS is not configured in the provider")
			Consistently(func(g Gomega) {
				g.Expect(testDevices.StateFor(deviceName).ISIS.Has("UNDERLAY")).To(BeFalse())
			}).Should(Succeed())
		})
	})
})
