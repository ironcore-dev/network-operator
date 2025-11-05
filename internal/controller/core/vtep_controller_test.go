// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openconfig/ygot/ygot"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nxosv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VTEP Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-vtep"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			vtep   *v1alpha1.VTEP
		)
		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}

			By("Ensuring loopback interfaces exist")
			for _, ifName := range []string{"lo0", "lo1"} {
				ifObj := &v1alpha1.Interface{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj); errors.IsNotFound(err) {
					ifObj = &v1alpha1.Interface{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ifName,
							Namespace: metav1.NamespaceDefault,
						},
						Spec: v1alpha1.InterfaceSpec{
							DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
							Name:       ifName,
							Type:       v1alpha1.InterfaceTypeLoopback,
							AdminState: "Up",
						},
					}
					Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
				}
			}

			By("Creating the custom resource for the Kind VTEP")
			vtep = &v1alpha1.VTEP{}
			if err := k8sClient.Get(ctx, key, vtep); errors.IsNotFound(err) {
				vtep = &v1alpha1.VTEP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.VTEPSpec{
						DeviceRef:           v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:         true,
						HostReachability:    "BGP",
						PrimaryInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo0"},
						AnycastInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo1"},
						MulticastGroup: &v1alpha1.MulticastGroup{
							Type:   v1alpha1.MulticastGroupTypeL2,
							Prefix: v1alpha1.IPPrefix{Prefix: netip.MustParsePrefix("234.1.1.0/24")},
						},
						Enabled: true,
					},
				}
				Expect(k8sClient.Create(ctx, vtep)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, vtep)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VTEP")
			Expect(k8sClient.Delete(ctx, vtep)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensure deviceinfo override is nil")
			Expect(testProvider.ResetDeviceInfoOverride()).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).To(BeNil(), "Provider VTEP should be empty")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(vtep, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.OwnerReferences).To(HaveLen(1))
				g.Expect(vtep.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(vtep.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.Status.Conditions).To(HaveLen(3))
				g.Expect(vtep.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(vtep.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(vtep.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(vtep.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(vtep.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(vtep.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the VTEP is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).ToNot(BeNil(), "Provider VTEP should not be nil")
				g.Expect(testProvider.VTEP.Spec.Enabled).To(BeTrue(), "Provider VTEP Enabled should be true")
				g.Expect(testProvider.VTEP.Spec.SuppressARP).To(BeTrue(), "Provider VTEP SuppressARP should be true")
				g.Expect(testProvider.VTEP.Spec.HostReachability).To(BeEquivalentTo("BGP"), "Provider VTEP hostreachability should be BGP")
				g.Expect(testProvider.VTEP.Spec.PrimaryInterfaceRef.Name).To(Equal("lo0"), "Provider VTEP primary interface should be lo0")
				g.Expect(testProvider.VTEP.Spec.AnycastInterfaceRef.Name).To(Equal("lo1"), "Provider VTEP anycast interface should be lo1")
				g.Expect(testProvider.VTEP.Spec.MulticastGroup).ToNot(BeNil(), "Provider VTEP multicast group should not be nil")
				g.Expect(testProvider.VTEP.Spec.MulticastGroup.Type).To(BeEquivalentTo(v1alpha1.MulticastGroupTypeL2), "Provider VTEP multicast group type should be L2")
				g.Expect(testProvider.VTEP.Spec.MulticastGroup.Prefix.String()).To(Equal("234.1.1.0/24"), "Provider VTEP multicast group prefix should be seet")
			}).Should(Succeed())

			By("Verifying referenced interfaces exist and are loopbacks")
			Eventually(func(g Gomega) {
				primary := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: vtep.Spec.PrimaryInterfaceRef.Name, Namespace: vtep.Namespace}, primary)).To(Succeed())
				g.Expect(primary.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(primary.Spec.DeviceRef.Name).To(Equal(name))

				anycast := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: vtep.Spec.AnycastInterfaceRef.Name, Namespace: vtep.Namespace}, anycast)).To(Succeed())
				g.Expect(anycast.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(anycast.Spec.DeviceRef.Name).To(Equal(name))
				g.Expect(anycast.Name).NotTo(Equal(primary.Name)) // ensure different interfaces
			}).Should(Succeed())

			By("Verifying the controller sets valid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.VTEP{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

	})

	Context("When using erroneous interface references (non loopback type)", func() {
		const name = "test-vtep-misconfigured-iftype"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			vtep   *v1alpha1.VTEP
		)

		BeforeEach(func() {
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Ensuring loopback interfaces with wrong type exist")
			for _, ifName := range []string{"eth1", "eth2"} {
				ifObj := &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ifName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypePhysical, // invalid for VTEP
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
			}

			By("Ensuring Cisco NXOS config (Kind VTEPConfig)")
			vtep = &v1alpha1.VTEP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VTEPSpec{
					DeviceRef:           v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:         true,
					HostReachability:    "BGP",
					PrimaryInterfaceRef: v1alpha1.LocalObjectReference{Name: "eth1"},
					AnycastInterfaceRef: v1alpha1.LocalObjectReference{Name: "eth2"},
					Enabled:             true,
				},
			}
			Expect(k8sClient.Create(ctx, vtep)).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, vtep)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VTEP")
			Expect(k8sClient.Delete(ctx, vtep)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Ensure deviceinfo override is nil")
			Expect(testProvider.ResetDeviceInfoOverride()).To(Succeed())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).To(BeNil(), "Provider VTEP should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with InvalidInterfaceTypeReason", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.VTEP{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				cond := findCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
			}).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (cross-device reference)", func() {
		const name = "test-vtep-misconfigured-crossdevice"
		const nameAlt = "test-vtep-misconfigured-crossdevice-alt" // device for interface reference
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device    *v1alpha1.Device
			deviceAlt *v1alpha1.Device
			vtep      *v1alpha1.VTEP
		)

		BeforeEach(func() {
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			deviceAlt = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameAlt,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, deviceAlt)).To(Succeed())

			By("Ensuring loopback interfaces with created on a different device")
			for _, ifName := range []string{"lo2", "lo3"} {
				ifObj := &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ifName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: nameAlt},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback, // invalid for VTEP
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
			}

			By("Ensuring Cisco NXOS config (Kind VTEPConfig)")
			vtep = &v1alpha1.VTEP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VTEPSpec{
					DeviceRef:           v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:         true,
					HostReachability:    "BGP",
					PrimaryInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo2"},
					AnycastInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo3"},
					Enabled:             true,
				},
			}
			Expect(k8sClient.Create(ctx, vtep)).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, vtep)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VTEP")
			Expect(k8sClient.Delete(ctx, vtep)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Ensure deviceinfo override is nil")
			Expect(testProvider.ResetDeviceInfoOverride()).To(Succeed())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, deviceAlt)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).To(BeNil(), "Provider VTEP should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with CrossDeviceReferenceReason", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.VTEP{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				cond := findCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})

	Context("When reconciling a resource with cisco nx provider ref", func() {
		const name = "test-vtep-with-prov-ref"
		const provConfigRefName = "test-vtepconfig-prov-ref-refname"

		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
		pcKey := client.ObjectKey{Name: provConfigRefName, Namespace: metav1.NamespaceDefault}

		var (
			device     *v1alpha1.Device
			vtep       *v1alpha1.VTEP
			vtepConfig *nxosv1alpha1.VTEPConfig
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				device = &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}
			Expect(testProvider.OverrideDeviceInfo(provider.DeviceInfo{
				Manufacturer:    "Cisco",
				FirmwareVersion: "10.4(3)",
				Model:           "N9K-C9300v",
				SerialNumber:    "123456789",
			})).To(Succeed())

			By("Ensuring loopback interfaces exist")
			for _, ifName := range []string{"lo4", "lo5"} {
				Eventually(func(g Gomega) {
					ifObj := &v1alpha1.Interface{}
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj); errors.IsNotFound(err) {
						ifObj = &v1alpha1.Interface{
							ObjectMeta: metav1.ObjectMeta{
								Name:      ifName,
								Namespace: metav1.NamespaceDefault,
							},
							Spec: v1alpha1.InterfaceSpec{
								DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
								Name:       ifName,
								Type:       v1alpha1.InterfaceTypeLoopback,
								AdminState: "Up",
							},
						}
						Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
					}
				}, 5*time.Second, 150*time.Millisecond).Should(Succeed())
			}

			By("Ensuring Cisco NXOS config (Kind VTEPConfig)")
			vtepConfig = &nxosv1alpha1.VTEPConfig{}
			if err := k8sClient.Get(ctx, pcKey, vtepConfig); errors.IsNotFound(err) {
				vtepConfig = &nxosv1alpha1.VTEPConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      provConfigRefName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxosv1alpha1.VTEPConfigSpec{
						HoldDownTime:  300,
						AdvertiseVMAC: ygot.Bool(true),
						InfraVLANs: []nxosv1alpha1.VLANListItem{
							{ID: 100},
							{RangeMin: 300, RangeMax: 400},
						},
					},
				}
				Expect(k8sClient.Create(ctx, vtepConfig)).To(Succeed())
			}

			By("Creating the custom resource for the Kind VTEP")
			vtep = &v1alpha1.VTEP{}
			if err := k8sClient.Get(ctx, key, vtep); errors.IsNotFound(err) {
				vtep = &v1alpha1.VTEP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.VTEPSpec{
						DeviceRef:           v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:         true,
						HostReachability:    "BGP",
						PrimaryInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo4"},
						AnycastInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo5"},
						MulticastGroup: &v1alpha1.MulticastGroup{
							Type:   v1alpha1.MulticastGroupTypeL2,
							Prefix: v1alpha1.IPPrefix{Prefix: netip.MustParsePrefix("234.1.1.0/24")},
						},
						ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
							APIVersion: nxosv1alpha1.GroupVersion.String(),
							Kind:       "VTEPConfig",
							Name:       provConfigRefName,
						},
						Enabled: true,
					},
				}
				Expect(k8sClient.Create(ctx, vtep)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, vtep)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VTEP")
			Expect(k8sClient.Delete(ctx, vtep)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Ensure deviceinfo override is nil")
			Expect(testProvider.ResetDeviceInfoOverride()).To(Succeed())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).To(BeNil(), "Provider VTEP should be empty")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(vtep, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.OwnerReferences).To(HaveLen(1))
				g.Expect(vtep.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(vtep.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vtep)).To(Succeed())
				g.Expect(vtep.Status.Conditions).To(HaveLen(3))
				g.Expect(vtep.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(vtep.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(vtep.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(vtep.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(vtep.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(vtep.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the VTEP is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VTEP).ToNot(BeNil(), "Provider VTEP should not be nil")
				g.Expect(testProvider.VTEP.Spec.ProviderConfigRef).ToNot(BeNil(), "Provider VTEP ProviderConfigRef should not be nil")
				g.Expect(testProvider.VTEP.Spec.ProviderConfigRef.APIVersion).To(Equal(nxosv1alpha1.GroupVersion.String()), "Provider VTEP ProviderConfigRef APIVersion should be set")
				g.Expect(testProvider.VTEP.Spec.ProviderConfigRef.Kind).To(Equal("VTEPConfig"), "Provider VTEP ProviderConfigRef Kind should be set")
				g.Expect(testProvider.VTEP.Spec.ProviderConfigRef.Name).To(Equal(provConfigRefName), "Provider VTEP ProviderConfigRef Name should be set")
			}).Should(Succeed())
		})

		It("Should fail if the device is not supported", func() {
			// Override provider device info with different model (simulate change)
			Expect(testProvider.OverrideDeviceInfo(provider.DeviceInfo{
				Manufacturer:    "Cisco",
				Model:           "N9K-C9400v", // changed model
				FirmwareVersion: "7.10",
				SerialNumber:    "123456789",
			})).To(Succeed())

			// Force device reconcile
			Eventually(func(g Gomega) {
				d := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, d)).To(Succeed())
				orig := d.DeepCopy()
				if d.Annotations == nil {
					d.Annotations = map[string]string{}
				}
				d.Annotations["test/reconcile-bump"] = time.Now().Format(time.RFC3339Nano)
				g.Expect(k8sClient.Patch(ctx, d, client.MergeFrom(orig))).To(Succeed())
			}).Should(Succeed())

			// Wait for device status update
			Eventually(func() string {
				d := &v1alpha1.Device{}
				err := k8sClient.Get(ctx, key, d)
				if err != nil {
					return ""
				}
				return d.Status.FirmwareVersion
			}, 5*time.Second, 200*time.Millisecond).Should(Equal("7.10"))

			// Expect VTEP to have Configured=False with IncompatibleProviderConfigRef
			Eventually(func(g Gomega) {
				v := &v1alpha1.VTEP{}
				g.Expect(k8sClient.Get(ctx, key, v)).To(Succeed())
				cond := findCondition(v.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})
	})
})

func findCondition(conds []metav1.Condition, condType string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == condType {
			return &conds[i]
		}
	}
	return nil
}
