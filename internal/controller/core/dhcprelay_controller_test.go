// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
			deviceName     string
			resourceName   string
			interfaceName  string
			vlanName       string
			resourceKey    client.ObjectKey
			deviceKey      client.ObjectKey
			interfaceKey   client.ObjectKey
			vlanKey        client.ObjectKey
			device         *v1alpha1.Device
			vlan           *v1alpha1.VLAN
			intf           *v1alpha1.Interface
			dhcprelay      *v1alpha1.DHCPRelay
			providerConfig *corev1.ConfigMap
		)

		BeforeEach(func() {
			providerConfig = nil

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
			dhcprelay.Name = resourceKey.Name
			dhcprelay.Namespace = resourceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dhcprelay))).To(Succeed())

			By("Waiting for DHCPRelay resource to be fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			if providerConfig != nil {
				By("Cleaning up the provider configuration resource")
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, providerConfig))).To(Succeed())
			}

			By("Cleaning up the Interface resource")
			intf = &v1alpha1.Interface{}
			intf.Name = interfaceKey.Name
			intf.Namespace = interfaceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, intf))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, interfaceKey, &v1alpha1.Interface{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the VLAN resource")
			vlan = &v1alpha1.VLAN{}
			vlan.Name = vlanKey.Name
			vlan.Namespace = vlanKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vlan))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, vlanKey, &v1alpha1.VLAN{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the resource has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).To(BeNil(), "Provider should have no DHCPRelay configured")
			}).Should(Succeed())

			By("Cleaning up the Device resource")
			device = &v1alpha1.Device{}
			device.Name = deviceKey.Name
			device.Namespace = deviceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind DHCPRelay")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1", "192.168.1.2"},
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

			By("Ensuring the DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil(), "Provider DHCPRelay should not be nil")
				if testProvider.DHCPRelay != nil {
					g.Expect(testProvider.DHCPRelay.GetName()).To(Equal(resourceName), "Provider should have DHCPRelay configured")
				}
			}).Should(Succeed())
		})

		It("Should reject an incompatible ProviderConfigRef", func() {
			By("Creating an unsupported provider configuration resource")
			providerConfig = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-config-",
					Namespace:    metav1.NamespaceDefault,
				},
			}
			Expect(k8sClient.Create(ctx, providerConfig)).To(Succeed())

			By("Creating a DHCPRelay that references the unsupported configuration")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       providerConfig.Name,
					},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the incompatible configuration is rejected")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
				g.Expect(testProvider.DHCPRelay).To(BeNil())
			}).Should(Succeed())
		})

		It("Should successfully reconcile using a top-level VRF", func() {
			By("Creating a VRF resource")
			vrf := &v1alpha1.VRF{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-vrf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VRFSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					Name:      "VRF-TEST",
				},
			}
			Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			vrfKey := client.ObjectKey{Name: vrf.Name, Namespace: metav1.NamespaceDefault}
			defer func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vrf))).To(Succeed())
			}()

			By("Waiting for VRF to be ready")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, vrfKey, vrf)).To(Succeed())
				cond := meta.FindStatusCondition(vrf.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating DHCPRelay using a top-level VRF")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					VrfRef:       &v1alpha1.LocalObjectReference{Name: vrf.Name},
					Servers:      []string{"192.168.1.1"},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceName = dhcprelay.Name
			resourceKey = client.ObjectKey{Name: resourceName, Namespace: metav1.NamespaceDefault}

			By("Verifying the controller sets ReadyCondition to True")
			Eventually(func(g Gomega) {
				dhcprelay = &v1alpha1.DHCPRelay{}
				g.Expect(k8sClient.Get(ctx, resourceKey, dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil())
				g.Expect(testProvider.DHCPRelay.GetName()).To(Equal(resourceName))
			}).Should(Succeed())
		})

		It("Should reject duplicate DHCPRelay resources on the same interface", func() {
			By("Creating the first DHCPRelay resource")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
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

			By("Creating a second DHCPRelay resource for the same interface")
			duplicateDHCPRelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-dup-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
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

		It("Should allow DHCPRelay resources on different interfaces of the same device", func() {
			By("Creating another VLAN and Interface resource")
			otherVLAN := &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-other-vlan-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName},
					ID:        11,
					Name:      "vlan11",
				},
			}
			Expect(k8sClient.Create(ctx, otherVLAN)).To(Succeed())
			defer func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherVLAN))).To(Succeed())
			}()

			otherInterface := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-other-intf-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
					Name:       "vlan11",
					Type:       v1alpha1.InterfaceTypeRoutedVLAN,
					AdminState: v1alpha1.AdminStateUp,
					VlanRef:    &v1alpha1.LocalObjectReference{Name: otherVLAN.Name},
					IPv4: &v1alpha1.InterfaceIPv4{
						Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.1.1/24")}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherInterface)).To(Succeed())
			defer func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherInterface))).To(Succeed())
			}()

			By("Waiting for the second Interface to be configured")
			otherInterfaceKey := client.ObjectKeyFromObject(otherInterface)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, otherInterfaceKey, otherInterface)).To(Succeed())
				cond := meta.FindStatusCondition(otherInterface.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating DHCPRelay resources for both interfaces")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-", Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
				},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			resourceKey = client.ObjectKeyFromObject(dhcprelay)

			otherDHCPRelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-other-", Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: otherInterface.Name},
					Servers:      []string{"192.168.1.1"},
				},
			}
			Expect(k8sClient.Create(ctx, otherDHCPRelay)).To(Succeed())
			otherDHCPRelayKey := client.ObjectKeyFromObject(otherDHCPRelay)
			defer func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherDHCPRelay))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(errors.IsNotFound(k8sClient.Get(ctx, otherDHCPRelayKey, &v1alpha1.DHCPRelay{}))).To(BeTrue())
				}).Should(Succeed())
			}()

			By("Verifying both DHCPRelay resources become ready")
			for _, key := range []client.ObjectKey{resourceKey, otherDHCPRelayKey} {
				Eventually(func(g Gomega) {
					relay := &v1alpha1.DHCPRelay{}
					g.Expect(k8sClient.Get(ctx, key, relay)).To(Succeed())
					cond := meta.FindStatusCondition(relay.Status.Conditions, v1alpha1.ReadyCondition)
					g.Expect(cond).ToNot(BeNil())
					g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			}
		})

		It("Should properly handle deletion and cleanup", func() {
			By("Creating the custom resource for the Kind DHCPRelay")
			dhcprelay = &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
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
			dhcprelay.Name = resourceKey.Name
			dhcprelay.Namespace = resourceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dhcprelay))).To(Succeed())
		})

		It("Should not add finalizer when Device does not exist", func() {
			By("Creating DHCPRelay referencing a non-existent Device")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-nodev-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: "non-existent-device"},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: "test-interface"},
					Servers:      []string{"192.168.1.1"},
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
			dhcprelay.Name = resourceKey.Name
			dhcprelay.Namespace = resourceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dhcprelay))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the unnumbered Interface resource")
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, unnumberedIntf))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, unnumberedIntfKey, &v1alpha1.Interface{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the loopback Interface resource")
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, loopbackIntf))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, loopbackIntfKey, &v1alpha1.Interface{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the provider has been cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).To(BeNil(), "Provider should have no DHCPRelay configured")
			}).Should(Succeed())

			By("Cleaning up the Device resource")
			device = &v1alpha1.Device{}
			device.Name = deviceKey.Name
			device.Namespace = deviceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should successfully reconcile with an unnumbered Interface", func() {
			By("Creating DHCPRelay with an unnumbered Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-unnum-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: unnumberedIntfName},
					Servers:      []string{"192.168.1.1"},
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

			By("Ensuring the DHCPRelay is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.DHCPRelay).ToNot(BeNil(), "Provider DHCPRelay should not be nil")
			}).Should(Succeed())
		})
	})

	Context("When the DHCPRelay references are invalid", func() {
		var (
			deviceName string
			deviceKey  client.ObjectKey
		)

		cleanupObject := func(object client.Object) {
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, object))).To(Succeed())
			})
		}

		cleanupDHCPRelay := func(dhcprelay *v1alpha1.DHCPRelay) {
			resourceKey := client.ObjectKeyFromObject(dhcprelay)
			DeferCleanup(func() {
				By("Cleaning up the DHCPRelay resource")
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dhcprelay))).To(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				}).Should(Succeed())
			})
		}

		BeforeEach(func() {
			By("Creating the Device resource")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-invalid-", Namespace: metav1.NamespaceDefault},
				Spec:       v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "192.168.10.51:9339"}},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			deviceName = device.Name
			deviceKey = client.ObjectKeyFromObject(device)
			DeferCleanup(func() {
				By("Cleaning up the Device resource")
				device := &v1alpha1.Device{ObjectMeta: metav1.ObjectMeta{Name: deviceKey.Name, Namespace: deviceKey.Namespace}}
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
			})
		})

		It("Should set ConfiguredCondition to False when Interface does not exist", func() {
			By("Creating DHCPRelay referencing a non-existent Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-noint-new-", Namespace: metav1.NamespaceDefault},
				Spec:       v1alpha1.DHCPRelaySpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, InterfaceRef: &v1alpha1.LocalObjectReference{Name: "non-existent-interface"}, Servers: []string{"192.168.1.1"}},
			}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			cleanupDHCPRelay(dhcprelay)

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dhcprelay), dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})

		It("Should set ConfiguredCondition to False when Interface belongs to a different Device", func() {
			By("Creating another Device resource")
			otherDevice := &v1alpha1.Device{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-crossdev-other-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "192.168.10.53:9339"}}}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())
			cleanupObject(otherDevice)

			By("Creating a VLAN on the other Device")
			otherVLAN := &v1alpha1.VLAN{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-crossdev-vlan-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.VLANSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: otherDevice.Name}, ID: 20, Name: "vlan20"}}
			Expect(k8sClient.Create(ctx, otherVLAN)).To(Succeed())
			cleanupObject(otherVLAN)

			By("Creating an Interface on the other Device")
			otherInterface := &v1alpha1.Interface{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-crossdev-intf-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.InterfaceSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: otherDevice.Name}, Name: "vlan20", Type: v1alpha1.InterfaceTypeRoutedVLAN, VlanRef: &v1alpha1.LocalObjectReference{Name: otherVLAN.Name}, AdminState: v1alpha1.AdminStateUp, IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.1.1/24")}}}}}
			Expect(k8sClient.Create(ctx, otherInterface)).To(Succeed())
			cleanupObject(otherInterface)

			By("Creating DHCPRelay referencing an Interface from a different device")
			dhcprelay := &v1alpha1.DHCPRelay{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-crossdev-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DHCPRelaySpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, InterfaceRef: &v1alpha1.LocalObjectReference{Name: otherInterface.Name}, Servers: []string{"192.168.1.1"}}}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			cleanupDHCPRelay(dhcprelay)

			By("Verifying the controller sets ConfiguredCondition to False with CrossDeviceReferenceReason")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dhcprelay), dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})

		It("Should set ConfiguredCondition to False when VRF belongs to a different Device", func() {
			By("Creating another Device resource")
			otherDevice := &v1alpha1.Device{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-vrfcross-other-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DeviceSpec{Endpoint: v1alpha1.Endpoint{Address: "192.168.10.58:9339"}}}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())
			cleanupObject(otherDevice)

			By("Creating a VLAN on the main Device")
			vlan := &v1alpha1.VLAN{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-vrfcross-vlan-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.VLANSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, ID: 60, Name: "vlan60"}}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			cleanupObject(vlan)

			By("Creating an Interface on the main Device")
			intf := &v1alpha1.Interface{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-vrfcross-intf-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.InterfaceSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, Name: "vlan60", Type: v1alpha1.InterfaceTypeRoutedVLAN, VlanRef: &v1alpha1.LocalObjectReference{Name: vlan.Name}, AdminState: v1alpha1.AdminStateUp, IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.6.1/24")}}}}}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			cleanupObject(intf)

			By("Waiting for Interface to be configured")
			interfaceKey := client.ObjectKeyFromObject(intf)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, interfaceKey, intf)).To(Succeed())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Creating a VRF on the other Device")
			otherVRF := &v1alpha1.VRF{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-vrfcross-vrf-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.VRFSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: otherDevice.Name}, Name: "VRF-OTHER"}}
			Expect(k8sClient.Create(ctx, otherVRF)).To(Succeed())
			cleanupObject(otherVRF)

			By("Creating DHCPRelay with a VRF from a different device")
			dhcprelay := &v1alpha1.DHCPRelay{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-vrfcross-new-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DHCPRelaySpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, InterfaceRef: &v1alpha1.LocalObjectReference{Name: intf.Name}, VrfRef: &v1alpha1.LocalObjectReference{Name: otherVRF.Name}, Servers: []string{"192.168.1.1"}}}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			cleanupDHCPRelay(dhcprelay)

			By("Verifying the controller sets ConfiguredCondition to False with CrossDeviceReferenceReason")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dhcprelay), dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(cond.Message).To(ContainSubstring("VRF"))
			}).Should(Succeed())
		})

		It("Should set ConfiguredCondition to False when Interface is not configured", func() {
			const nonExistentVrfName = "testdhcprelay-intfnotready-nonexistent-vrf"
			By("Creating the VLAN resource")
			vlan := &v1alpha1.VLAN{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-intfnr-vlan-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.VLANSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, ID: 40, Name: "vlan40", AdminState: v1alpha1.AdminStateUp}}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())
			cleanupObject(vlan)

			By("Creating an Interface resource with a VRF reference to a non-existent VRF")
			intf := &v1alpha1.Interface{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-intfnr-intf-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.InterfaceSpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, Name: "vlan40", AdminState: v1alpha1.AdminStateUp, Type: v1alpha1.InterfaceTypeRoutedVLAN, VlanRef: &v1alpha1.LocalObjectReference{Name: vlan.Name}, VrfRef: &v1alpha1.LocalObjectReference{Name: nonExistentVrfName}, IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{{Prefix: netip.MustParsePrefix("10.0.4.1/24")}}}}}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())
			cleanupObject(intf)

			By("Verifying the Interface is NOT Ready")
			interfaceKey := client.ObjectKeyFromObject(intf)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, interfaceKey, intf)).To(Succeed())
				cond := meta.FindStatusCondition(intf.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Creating DHCPRelay referencing a non-configured Interface")
			dhcprelay := &v1alpha1.DHCPRelay{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-dhcprelay-intfnr-", Namespace: metav1.NamespaceDefault}, Spec: v1alpha1.DHCPRelaySpec{DeviceRef: v1alpha1.LocalObjectReference{Name: deviceName}, InterfaceRef: &v1alpha1.LocalObjectReference{Name: intf.Name}, Servers: []string{"192.168.1.1"}}}
			Expect(k8sClient.Create(ctx, dhcprelay)).To(Succeed())
			cleanupDHCPRelay(dhcprelay)

			By("Verifying the controller sets ConfiguredCondition to False with WaitingForDependenciesReason")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dhcprelay), dhcprelay)).To(Succeed())
				cond := meta.FindStatusCondition(dhcprelay.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
				g.Expect(cond.Message).To(ContainSubstring("not configured"))
			}).Should(Succeed())
		})
	})

	Context("When Interface becomes configured", func() {
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
			dhcprelay.Name = resourceKey.Name
			dhcprelay.Namespace = resourceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dhcprelay))).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, resourceKey, &v1alpha1.DHCPRelay{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the Interface resource")
			i := &v1alpha1.Interface{}
			i.Name = interfaceKey.Name
			i.Namespace = interfaceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, i))).To(Succeed())

			By("Cleaning up the VLAN resource")
			vlan := &v1alpha1.VLAN{}
			vlan.Name = vlanKey.Name
			vlan.Namespace = vlanKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, vlan))).To(Succeed())

			By("Cleaning up the Device resource")
			device := &v1alpha1.Device{}
			device.Name = deviceKey.Name
			device.Namespace = deviceKey.Namespace
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should re-reconcile DHCPRelay when Interface becomes configured (watch trigger)", func() {
			By("Creating DHCPRelay referencing a non-configured Interface")
			dhcprelay := &v1alpha1.DHCPRelay{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-dhcprelay-intfnr-watch-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DHCPRelaySpec{
					DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
					InterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceName},
					Servers:      []string{"192.168.1.1"},
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
