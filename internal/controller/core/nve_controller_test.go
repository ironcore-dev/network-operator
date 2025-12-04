// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NVE Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-nve"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			nve    *v1alpha1.NVE
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

			By("Creating the custom resource for the Kind NVE")
			nve = &v1alpha1.NVE{}
			if err := k8sClient.Get(ctx, key, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NVE{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.NVESpec{
						DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:               true,
						HostReachability:          "BGP",
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "lo0"},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo1"},
						MulticastGroups: &v1alpha1.MulticastGroups{
							L2: "234.0.0.1",
						},
						AdminState: v1alpha1.AdminStateUp,
					},
				}
				Expect(k8sClient.Create(ctx, nve)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, nve)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NVE")
			Expect(k8sClient.Delete(ctx, nve)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, nve)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(nve, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, nve)).To(Succeed())
				g.Expect(nve.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, nve)).To(Succeed())
				g.Expect(nve.OwnerReferences).To(HaveLen(1))
				g.Expect(nve.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(nve.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, nve)).To(Succeed())
				g.Expect(nve.Status.Conditions).To(HaveLen(3))
				g.Expect(nve.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(nve.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(nve.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(nve.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(nve.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(nve.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the NVE is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).ToNot(BeNil(), "Provider NVE should not be nil")
				g.Expect(testProvider.NVE.Spec.AdminState).To(BeEquivalentTo(v1alpha1.AdminStateUp), "Provider NVE Enabled should be true")
				g.Expect(testProvider.NVE.Spec.SuppressARP).To(BeTrue(), "Provider NVE SuppressARP should be true")
				g.Expect(testProvider.NVE.Spec.HostReachability).To(BeEquivalentTo("BGP"), "Provider NVE hostreachability should be BGP")
				g.Expect(testProvider.NVE.Spec.SourceInterfaceRef.Name).To(Equal("lo0"), "Provider NVE primary interface should be lo0")
				g.Expect(testProvider.NVE.Spec.MulticastGroups).ToNot(BeNil(), "Provider NVE multicast group should not be nil")
				g.Expect(testProvider.NVE.Spec.MulticastGroups.L2).To(Equal("234.0.0.1"), "Provider NVE multicast group prefix should be seet")
			}).Should(Succeed())

			By("Verifying referenced interfaces exist and are loopbacks")
			Eventually(func(g Gomega) {
				primary := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.SourceInterfaceRef.Name, Namespace: nve.Namespace}, primary)).To(Succeed())
				g.Expect(primary.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(primary.Spec.DeviceRef.Name).To(Equal(name))

				anycast := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.AnycastSourceInterfaceRef.Name, Namespace: nve.Namespace}, anycast)).To(Succeed())
				g.Expect(anycast.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(anycast.Spec.DeviceRef.Name).To(Equal(name))
				g.Expect(anycast.Name).NotTo(Equal(primary.Name)) // ensure different interfaces
			}).Should(Succeed())

			By("Verifying the controller sets valid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.NVE{}
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

	Context("When updating referenced resources", func() {
		const name = "test-nve-with-ref-updates"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			nve    *v1alpha1.NVE
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
			for _, ifName := range []string{"lo10", "lo11"} {
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

			By("Creating the custom resource for the Kind NVE")
			nve = &v1alpha1.NVE{}
			if err := k8sClient.Get(ctx, key, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NVE{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.NVESpec{
						DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:        true,
						HostReachability:   "BGP",
						SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo10"},
						// AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo11"},
						MulticastGroups: &v1alpha1.MulticastGroups{
							L2: "234.0.0.1",
						},
						AdminState: v1alpha1.AdminStateUp,
					},
				}
				Expect(k8sClient.Create(ctx, nve)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance NVE")
			Expect(k8sClient.Delete(ctx, nve)).To(Succeed())

			By("Deleting loopback interfaces")
			for _, ifName := range []string{"lo10", "lo11", "lo12"} {
				ifObj := &v1alpha1.Interface{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj)
				if err == nil {
					Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
					Eventually(func() bool {
						err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj)
						return errors.IsNotFound(err)
					}).Should(BeTrue())
				}
			}

			By("Deleting all Kind Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, device)
				return errors.IsNotFound(err)
			}).Should(BeTrue())
		})

		It("Should reconcile when SourceInterfaceRef is changed", func() {
			By("Patching NVE to update SourceInterfaceRef")
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.SourceInterfaceRef = v1alpha1.LocalObjectReference{Name: "lo11"}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Verifying reconciliation updates provider and status")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).ToNot(BeNil())
				g.Expect(testProvider.NVE.Spec.SourceInterfaceRef.Name).To(Equal("lo11"))
				g.Expect(testProvider.NVE.Status.SourceInterfaceName).To(Equal("lo11"))
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})

		It("Should reconcile when AnycastSourceInterfaceRef is added", func() {
			By("Patching NVE to add AnycastSourceInterfaceRef")
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.AnycastSourceInterfaceRef = &v1alpha1.LocalObjectReference{Name: "lo12"}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Creating the anycast interface")
			ifObj := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lo12",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       "lo12",
					Type:       v1alpha1.InterfaceTypeLoopback,
					AdminState: "Up",
				},
			}
			Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())

			By("Verifying reconciliation updates provider and status")
			Eventually(func(g Gomega) {
				if testProvider.NVE != nil {
					g.Expect(testProvider.NVE).ToNot(BeNil())
					g.Expect(testProvider.NVE.Spec.AnycastSourceInterfaceRef.Name).To(Equal("lo12"))
					g.Expect(testProvider.NVE.Status.AnycastSourceInterfaceName).To(Equal("lo12"))
				}
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (non loopback type)", func() {
		const name = "test-nve-misconfigured-iftype"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			nve    *v1alpha1.NVE
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
						Type:       v1alpha1.InterfaceTypePhysical, // invalid for NVE
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
			}

			By("Creating the custom resource for the Kind NVE")
			nve = &v1alpha1.NVE{}
			if err := k8sClient.Get(ctx, key, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NVE{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.NVESpec{
						DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:               true,
						HostReachability:          "BGP",
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "eth1"},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "eth2"},
						MulticastGroups: &v1alpha1.MulticastGroups{
							L2: "234.0.0.1",
						},
						AdminState: v1alpha1.AdminStateUp,
					},
				}
				Expect(k8sClient.Create(ctx, nve)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, nve)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NVE")
			Expect(k8sClient.Delete(ctx, nve)).To(Succeed())

			By("Cleanup the specific resource instance Interfaces")
			for _, ifName := range []string{"eth1", "eth2"} {
				ifObj := &v1alpha1.Interface{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
			}

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with InvalidInterfaceTypeReason", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.NVE{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
			}).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (cross-device reference)", func() {
		const name = "test-nve-misconfigured-crossdevice"
		const nameAlt = "test-nve-misconfigured-crossdevice-alt" // device for interface reference
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device    *v1alpha1.Device
			deviceAlt *v1alpha1.Device
			nve       *v1alpha1.NVE
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
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
			}

			By("Creating the custom resource for the Kind NVE")
			nve = &v1alpha1.NVE{}
			if err := k8sClient.Get(ctx, key, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NVE{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.NVESpec{
						DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
						SuppressARP:               true,
						HostReachability:          "BGP",
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "lo2"},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo3"},
						MulticastGroups: &v1alpha1.MulticastGroups{
							L2: "234.0.0.1",
						},
						AdminState: v1alpha1.AdminStateUp,
					},
				}
				Expect(k8sClient.Create(ctx, nve)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, nve)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NVE")
			Expect(k8sClient.Delete(ctx, nve)).To(Succeed())

			By("Cleanup the specific resource instance Interfaces")
			for _, ifName := range []string{"lo2", "lo3"} {
				ifObj := &v1alpha1.Interface{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: ifName, Namespace: metav1.NamespaceDefault}, ifObj)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
			}

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Cleanup the specific resource second instance Device")
			Expect(k8sClient.Delete(ctx, deviceAlt)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with CrossDeviceReferenceReason", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.NVE{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})

	Context("When using a non registered dependency for providerConfigRef", func() {
		const name = "test-nve-misconfigured-providerconfigref"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			nve    *v1alpha1.NVE
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

			By("Ensuring loopback interfaces with created on a different device")
			for _, ifName := range []string{"lo6", "lo7", "lo8"} {
				ifObj := &v1alpha1.Interface{
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

			By("Ensuring Cisco NXOS config (Kind NVOConfig)")
			nve = &v1alpha1.NVE{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.NVESpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "lo6"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo7"},
					AdminState:                v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						Name:       "lo8",
						Kind:       "Interface",
						APIVersion: "networking.metal.ironcore.dev/v1alpha1",
					}, // invalid provider config ref
				},
			}
			Expect(k8sClient.Create(ctx, nve)).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, nve)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NVE")
			Expect(k8sClient.Delete(ctx, nve)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with IncompatibleProviderConfigRef", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.NVE{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})
	})
})
