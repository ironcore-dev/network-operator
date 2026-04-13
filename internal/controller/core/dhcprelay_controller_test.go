// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("DHCPRelay Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			deviceName    string
			resourceName  string
			interfaceName string
			vlanName      string
			resourceKey   client.ObjectKey
			deviceKey     client.ObjectKey
			interfaceKey  client.ObjectKey
			vlanKey       client.ObjectKey
			device        *v1alpha1.Device
			vlan          *v1alpha1.VLAN
			intf          *v1alpha1.Interface
			dhcprelay     *v1alpha1.DHCPRelay
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.50:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind VLAN")
			vlan = &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vlan-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					ID:        10,
					Name:      "vlan10",
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			vlanName = vlan.Name
			vlanKey = client.ObjectKey{Name: vlanName, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind Interface")
			intf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "vlan10",
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					AdminState: v1alpha1.AdminStateUp,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: vlanName},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.0.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			interfaceName = intf.Name
			interfaceKey = client.ObjectKey{Name: interfaceName, Namespace: metav1.NamespaceDefault}

			By("Waiting for Interface to be configured")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, interfaceKey, intf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay = &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())

				By("Waiting for DHCPRelay resource to be fully deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Interface resource")
			intf = &v1alpha1.Interface{}
			err = k8sClient.Get(ctx, interfaceKey, intf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, interfaceKey, &v1alpha1.Interface{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the VLAN resource")
			vlan = &v1alpha1.VLAN{}
			err = k8sClient.Get(ctx, vlanKey, vlan)
			if err == nil {
				Expect(k8sClient.Delete(ctx, vlan)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, vlanKey, &v1alpha1.VLAN{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).To(BeNil(), "Provider should have no DHCPRelay configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind DHCPRelay")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1", "192.168.1.2"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(dhcprelay, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				g.Expect(dhcprelay.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, deviceName))
			}).Should(Succeed())

			By("Verifying the controller sets the owner reference")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				g.Expect(dhcprelay.OwnerReferences).To(HaveLen(1))
				g.Expect(dhcprelay.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(dhcprelay.OwnerReferences[0].Name).To(Equal(deviceName))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the status contains configured interface refs")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				g.Expect(dhcprelay.Status.ConfiguredInterfaces).To(ContainElement(intf.Spec.Name))
			}).Should(Succeed())

			By("Ensuring the DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil(), "Provider DHCPRelay should not be nil")
				if testProvider.DHCPRelay != nil {
					g.Expect(testProvider.DHCPRelay.GetName()).To(Equal(resourceName), "Provider should have DHCPRelay configured")
				}
			}).Should(Succeed())
		})

		It("Should reject duplicate DHCPRelay resources on the same device", func() {
			By("Creating the first DHCPRelay resource")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Waiting for the first DHCPRelay to be ready")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating a second DHCPRelay resource for the same device")
			duplicateDHCPRelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-dup-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, duplicateDHCPRelay)).To(Succeed())
			duplicateKey := client.ObjectKey{Name: duplicateDHCPRelay.Name, Namespace: metav1.NamespaceDefault}

			By("Verifying the second DHCPRelay has a ConfiguredCondition=False with DuplicateResourceOnDevice reason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, duplicateKey, duplicateDHCPRelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(duplicateDHCPRelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.DuplicateResourceOnDevice))
			}).Should(Succeed())

			By("Cleaning up the duplicate DHCPRelay resource")
			Expect(k8sClient.Delete(ctx, duplicateDHCPRelay)).To(Succeed())
		})

		It("Should properly handle deletion and cleanup", func() {
			By("Creating the custom resource for the Kind DHCPRelay")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Waiting for the DHCPRelay to be ready")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil())
			}).Should(Succeed())

			By("Deleting the DHCPRelay resource")
			Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())

			By("Verifying the DHCPRelay is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).To(BeNil(), "Provider should have no DHCPRelay configured after deletion")
			}).Should(Succeed())

			By("Verifying the resource is fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	Context("When DeviceRef references non-existent Device", func() {
		var (
			resourceName string
			resourceKey  client.ObjectKey
		)

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				// Remove finalizer if present to allow deletion
				if controllerutil.ContainsFinalizer(dhcprelay, v1alpha1.FinalizerName) {
					controllerutil.RemoveFinalizer(dhcprelay, v1alpha1.FinalizerName)
					Expect(k8sClient.Update(ctx, dhcprelay)).To(Succeed())
				}
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
			}
		})

		It("Should not add finalizer when Device does not exist", func() {
			By("Creating DHCPRelay referencing a non-existent Device")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-nodev-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: "non-existent-device"},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: "test-interface"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller does not add a finalizer")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(controllerutil.ContainsFinalizer(dhcprelay, v1alpha1.FinalizerName)).To(BeFalse())
			}).Should(Succeed())
		})
	})

	Context("When InterfaceRef references non-existent Interface", func() {
		var (
			deviceName   string
			resourceName string
			resourceKey  client.ObjectKey
			deviceKey    client.ObjectKey
			device       *v1alpha1.Device
		)

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-noint-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.51:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
		})

		It("Should set ConfiguredCondition to False when Interface does not exist", func() {
			By("Creating DHCPRelay referencing a non-existent Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-noint-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: "non-existent-interface"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})
	})

	Context("When InterfaceRef belongs to a different device", func() {
		var (
			deviceName      string
			otherDeviceName string
			resourceName    string
			otherIntfName   string
			otherVlanName   string
			resourceKey     client.ObjectKey
			deviceKey       client.ObjectKey
			otherDeviceKey  client.ObjectKey
			otherIntfKey    client.ObjectKey
			otherVlanKey    client.ObjectKey
			device          *v1alpha1.Device
			otherDevice     *v1alpha1.Device
			otherVlan       *v1alpha1.VLAN
			otherIntf       *v1alpha1.Interface
		)

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-crossdev-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.52:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

			By("Creating another Device resource")
			otherDevice = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-crossdev-other-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.53:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())
			otherDeviceName = otherDevice.Name
			otherDeviceKey = client.ObjectKey{Name: otherDeviceName, Namespace: metav1.NamespaceDefault}

			By("Creating a VLAN on the other Device")
			otherVlan = &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-crossdev-vlan-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: otherDeviceName},
					ID:        20,
					Name:      "vlan20",
				},
			}
			Expect(k8sClient.Create(ctx, otherVlan)).To(Succeed())
			otherVlanName = otherVlan.Name
			otherVlanKey = client.ObjectKey{Name: otherVlanName, Namespace: metav1.NamespaceDefault}

			By("Creating an Interface on the other Device")
			otherIntf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-crossdev-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: otherDeviceName},
					Name:       "vlan20",
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: otherVlanName},
					AdminState: v1alpha1.AdminStateUp,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.1.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherIntf)).To(Succeed())
			otherIntfName = otherIntf.Name
			otherIntfKey = client.ObjectKey{Name: otherIntfName, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Interface resource")
			err = k8sClient.Get(ctx, otherIntfKey, otherIntf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, otherIntf)).To(Succeed())
			}

			By("Cleaning up the VLAN resource")
			err = k8sClient.Get(ctx, otherVlanKey, otherVlan)
			if err == nil {
				Expect(k8sClient.Delete(ctx, otherVlan)).To(Succeed())
			}

			By("Cleaning up the Device resources")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
			err = k8sClient.Get(ctx, otherDeviceKey, otherDevice)
			if err == nil {
				Expect(k8sClient.Delete(ctx, otherDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
		})

		It("Should set ConfiguredCondition to False with CrossDeviceReferenceReason", func() {
			By("Creating DHCPRelay referencing an Interface from a different device")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-crossdev-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: otherIntfName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ConfiguredCondition to False with CrossDeviceReferenceReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})

	Context("When VrfRef belongs to a different device", func() {
		var (
			deviceName      string
			otherDeviceName string
			resourceName    string
			interfaceName   string
			vlanName        string
			otherVrfName    string
			resourceKey     client.ObjectKey
			deviceKey       client.ObjectKey
			otherDeviceKey  client.ObjectKey
			interfaceKey    client.ObjectKey
			vlanKey         client.ObjectKey
			otherVrfKey     client.ObjectKey
			device          *v1alpha1.Device
			otherDevice     *v1alpha1.Device
			vlan            *v1alpha1.VLAN
			intf            *v1alpha1.Interface
			otherVrf        *v1alpha1.VRF
		)

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.57:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

			By("Creating another Device resource")
			otherDevice = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-other-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.58:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())
			otherDeviceName = otherDevice.Name
			otherDeviceKey = client.ObjectKey{Name: otherDeviceName, Namespace: metav1.NamespaceDefault}

			By("Creating a VLAN on the main Device")
			vlan = &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-vlan-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					ID:        60,
					Name:      "vlan60",
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			vlanName = vlan.Name
			vlanKey = client.ObjectKey{Name: vlanName, Namespace: metav1.NamespaceDefault}

			By("Creating an Interface on the main Device")
			intf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "vlan60",
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: vlanName},
					AdminState: v1alpha1.AdminStateUp,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.6.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			interfaceName = intf.Name
			interfaceKey = client.ObjectKey{Name: interfaceName, Namespace: metav1.NamespaceDefault}

			By("Waiting for Interface to be configured")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, interfaceKey, intf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating a VRF on the other Device")
			otherVrf = &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-vrf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: otherDeviceName},
					Name:      "VRF-OTHER",
				},
			}
			Expect(k8sClient.Create(ctx, otherVrf)).To(Succeed())
			otherVrfName = otherVrf.Name
			otherVrfKey = client.ObjectKey{Name: otherVrfName, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the VRF resource")
			err = k8sClient.Get(ctx, otherVrfKey, otherVrf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, otherVrf)).To(Succeed())
			}

			By("Cleaning up the Interface resource")
			err = k8sClient.Get(ctx, interfaceKey, intf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
			}

			By("Cleaning up the VLAN resource")
			err = k8sClient.Get(ctx, vlanKey, vlan)
			if err == nil {
				Expect(k8sClient.Delete(ctx, vlan)).To(Succeed())
			}

			By("Cleaning up the Device resources")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
			err = k8sClient.Get(ctx, otherDeviceKey, otherDevice)
			if err == nil {
				Expect(k8sClient.Delete(ctx, otherDevice, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
		})

		It("Should set ConfiguredCondition to False with CrossDeviceReferenceReason", func() {
			By("Creating DHCPRelay referencing a VRF from a different device")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrfcross-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
					VrfRef: &v1alpha1.LocalObjectReference{Name: otherVrfName},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ConfiguredCondition to False with CrossDeviceReferenceReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(cond.Message).To(ContainSubstring("VRF"))
			}).Should(Succeed())
		})
	})

	Context("When Interface has unnumbered IPv4 configuration", func() {
		var (
			deviceName         string
			resourceName       string
			loopbackIntfName   string
			unnumberedIntfName string
			resourceKey        client.ObjectKey
			deviceKey          client.ObjectKey
			loopbackIntfKey    client.ObjectKey
			unnumberedIntfKey  client.ObjectKey
			device             *v1alpha1.Device
			loopbackIntf       *v1alpha1.Interface
			unnumberedIntf     *v1alpha1.Interface
		)

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-unnum-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.54:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

			By("Creating a loopback Interface with an IP address")
			loopbackIntf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-unnum-lo-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "loopback0",
					Type:       v1alpha1.InterfaceTypeLoopback,
					AdminState: v1alpha1.AdminStateUp,
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.255.255.1/32")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, loopbackIntf)).To(Succeed())
			loopbackIntfName = loopbackIntf.Name
			loopbackIntfKey = client.ObjectKey{Name: loopbackIntfName, Namespace: metav1.NamespaceDefault}

			By("Waiting for loopback Interface to be ready")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, loopbackIntfKey, loopbackIntf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(loopbackIntf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating an unnumbered Interface referencing the loopback")
			unnumberedIntf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-unnum-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "ethernet1/1",
					Type:       v1alpha1.InterfaceTypePhysical,
					AdminState: v1alpha1.AdminStateUp,
					IPv4: &v1alpha1.InterfaceIPv4{
						Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
							InterfaceRef: v1alpha1.LocalObjectReference{Name: loopbackIntfName},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, unnumberedIntf)).To(Succeed())
			unnumberedIntfName = unnumberedIntf.Name
			unnumberedIntfKey = client.ObjectKey{Name: unnumberedIntfName, Namespace: metav1.NamespaceDefault}

			By("Waiting for unnumbered Interface to be configured")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, unnumberedIntfKey, unnumberedIntf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(unnumberedIntf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the unnumbered Interface resource")
			err = k8sClient.Get(ctx, unnumberedIntfKey, unnumberedIntf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, unnumberedIntf)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, unnumberedIntfKey, &v1alpha1.Interface{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the loopback Interface resource")
			err = k8sClient.Get(ctx, loopbackIntfKey, loopbackIntf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, loopbackIntf)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, loopbackIntfKey, &v1alpha1.Interface{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Device resource")
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}

			By("Verifying the provider has been cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).To(BeNil(), "Provider should have no DHCPRelay configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile with an unnumbered Interface", func() {
			By("Creating DHCPRelay with an unnumbered Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-unnum-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: unnumberedIntfName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ReadyCondition to True")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the status contains configured interface refs")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dhcprelay.Status.ConfiguredInterfaces).To(ContainElement(unnumberedIntf.Spec.Name))
			}).Should(Succeed())

			By("Ensuring the DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil(), "Provider DHCPRelay should not be nil")
			}).Should(Succeed())
		})
	})

	Context("When Interface is not Ready", func() {
		var (
			deviceName    string
			resourceName  string
			interfaceName string
			vlanName      string
			resourceKey   client.ObjectKey
			deviceKey     client.ObjectKey
			interfaceKey  client.ObjectKey
			vlanKey       client.ObjectKey
			device        *v1alpha1.Device
			vlan          *v1alpha1.VLAN
			intf          *v1alpha1.Interface
		)

		const nonExistentVrfName = "testdhcprelay-intfnotready-nonexistent-vrf"

		BeforeEach(func() {
			By("Creating the Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.55:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}

			By("Creating the VLAN resource")
			vlan = &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-vlan-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					ID:         40,
					Name:       "vlan40",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			vlanName = vlan.Name
			vlanKey = client.ObjectKey{Name: vlanName, Namespace: metav1.NamespaceDefault}

			By("Creating the Interface resource with a VRF reference to a non-existent VRF (will not become Ready)")
			intf = &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "vlan40",
					AdminState: v1alpha1.AdminStateUp,
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: vlanName},
					VrfRef:     &v1alpha1.LocalObjectReference{Name: nonExistentVrfName},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.4.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			interfaceName = intf.Name
			interfaceKey = client.ObjectKey{Name: interfaceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the Interface is NOT Ready (because VRF doesn't exist)")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, interfaceKey, intf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the DHCPRelay resource")
			dhcprelay := &v1alpha1.DHCPRelay{}
			err := k8sClient.Get(ctx, resourceKey, dhcprelay)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dhcprelay)).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			}

			By("Cleaning up the Interface resource")
			err = k8sClient.Get(ctx, interfaceKey, intf)
			if err == nil {
				Expect(k8sClient.Delete(ctx, intf)).To(Succeed())
			}

			By("Cleaning up the VLAN resource")
			err = k8sClient.Get(ctx, vlanKey, vlan)
			if err == nil {
				Expect(k8sClient.Delete(ctx, vlan)).To(Succeed())
			}

			By("Cleaning up the Device resource")
			device = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, deviceKey, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			}
		})

		It("Should set ConfiguredCondition to False with WaitingForDependenciesReason when Interface is not configured", func() {
			By("Creating DHCPRelay referencing a non-configured Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
				g.Expect(cond.Message).To(ContainSubstring("not configured"))
			}).Should(Succeed())
		})

		It("Should re-reconcile DHCPRelay when Interface becomes configured (watch trigger)", func() {
			By("Creating DHCPRelay referencing a non-configured Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-watch-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Servers:   []string{"192.168.1.1"},
					InterfaceRefs: []v1alpha1.LocalObjectReference{
						{Name: interfaceName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying DHCPRelay is not ready due to non-configured Interface")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())

			By("Creating the VRF to make the Interface configured")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nonExistentVrfName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Name:      "VRF-TEST",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())

			By("Waiting for Interface to become configured")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, interfaceKey, intf)
				g.Expect(err).NotTo(HaveOccurred())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying DHCPRelay becomes ready after Interface is configured (watch triggered re-reconciliation)")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, dhcprelay)
				g.Expect(err).NotTo(HaveOccurred())

				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Cleaning up the VRF resource")
			Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())
		})
	})
})
