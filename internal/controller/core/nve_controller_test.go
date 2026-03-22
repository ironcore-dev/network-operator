// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

const testEndpointAddr = "192.168.10.2:9339"

var _ = Describe("NVE Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name          string
			nveKey        client.ObjectKey
			deviceKey     client.ObjectKey
			interfaceKeys []client.ObjectKey
			nve           *v1alpha1.NetworkVirtualizationEdge
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating loopback interfaces")
			for _, ifName := range []string{name + "-lo0", name + "-lo1"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}
			interfaceKeys = []client.ObjectKey{
				{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo1", Namespace: metav1.NamespaceDefault},
			}

			By("Creating the custom resource for the Kind NVE")
			l2Prefix := v1alpha1.MustParsePrefix("234.0.0.0/8")
			nve = &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: name + "-lo1"},
					MulticastGroups:           &v1alpha1.MulticastGroups{L2: &l2Prefix},
					AdminState:                v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, nve)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interfaces")
			for _, ifKey := range interfaceKeys {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(nve, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				g.Expect(nve.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				g.Expect(nve.OwnerReferences).To(HaveLen(1))
				g.Expect(nve.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(nve.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				g.Expect(nve.Status.Conditions).To(HaveLen(4))
				g.Expect(nve.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(nve.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(nve.Status.Conditions[1].Type).To(Equal(v1alpha1.ConfiguredCondition))
				g.Expect(nve.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(nve.Status.Conditions[2].Type).To(Equal(v1alpha1.OperationalCondition))
				g.Expect(nve.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(nve.Status.Conditions[3].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(nve.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the NVE is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).ToNot(BeNil(), "Provider NVE should not be nil")
				g.Expect(testProvider.NVE.Spec.AdminState).To(BeEquivalentTo(v1alpha1.AdminStateUp))
				g.Expect(testProvider.NVE.Spec.SuppressARP).To(BeTrue())
				g.Expect(testProvider.NVE.Spec.HostReachability).To(BeEquivalentTo("BGP"))
				g.Expect(testProvider.NVE.Spec.SourceInterfaceRef.Name).To(Equal(name + "-lo0"))
				g.Expect(testProvider.NVE.Spec.MulticastGroups).ToNot(BeNil())
				g.Expect(testProvider.NVE.Spec.MulticastGroups.L2).To(HaveValue(Equal(v1alpha1.MustParsePrefix("234.0.0.0/8"))))
			}).Should(Succeed())

			By("Verifying referenced interfaces exist and are loopbacks")
			Eventually(func(g Gomega) {
				primary := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.SourceInterfaceRef.Name, Namespace: metav1.NamespaceDefault}, primary)).To(Succeed())
				g.Expect(primary.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(primary.Spec.DeviceRef.Name).To(Equal(name))

				anycast := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.AnycastSourceInterfaceRef.Name, Namespace: metav1.NamespaceDefault}, anycast)).To(Succeed())
				g.Expect(anycast.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(anycast.Spec.DeviceRef.Name).To(Equal(name))
				g.Expect(anycast.Name).NotTo(Equal(primary.Name))
			}).Should(Succeed())

			By("Verifying the controller sets valid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, resource)).To(Succeed())
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
	})

	Context("When updating referenced resources", func() {
		var (
			name          string
			nveKey        client.ObjectKey
			deviceKey     client.ObjectKey
			interfaceKeys []client.ObjectKey
			nve           *v1alpha1.NetworkVirtualizationEdge
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-refupdates-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating loopback interfaces")
			for _, ifName := range []string{name + "-lo0", name + "-lo1", name + "-lo2"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}
			interfaceKeys = []client.ObjectKey{
				{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo1", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo2", Namespace: metav1.NamespaceDefault},
			}

			By("Creating the custom resource for the Kind NVE")
			l2Prefix := v1alpha1.MustParsePrefix("234.0.0.0/8")
			nve = &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:        true,
					HostReachability:   "BGP",
					SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					MulticastGroups:    &v1alpha1.MulticastGroups{L2: &l2Prefix},
					AdminState:         v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, nve)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interfaces")
			for _, ifKey := range interfaceKeys {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should reconcile when SourceInterfaceRef is changed", func() {
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.SourceInterfaceRef = v1alpha1.LocalObjectReference{Name: name + "-lo1"}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Verifying reconciliation modifies provider and status")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).ToNot(BeNil())
				g.Expect(testProvider.NVE.Spec.SourceInterfaceRef.Name).To(Equal(name + "-lo1"))
				g.Expect(testProvider.NVE.Status.SourceInterfaceName).To(Equal(name + "-lo1"))
			}).Should(Succeed())
		})

		It("Should reconcile when AnycastSourceInterfaceRef is added", func() {
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.AnycastSourceInterfaceRef = &v1alpha1.LocalObjectReference{Name: name + "-lo2"}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Verifying reconciliation modifies provider and status")
			Eventually(func(g Gomega) {
				if testProvider.NVE != nil {
					g.Expect(testProvider.NVE).ToNot(BeNil())
					g.Expect(testProvider.NVE.Spec.AnycastSourceInterfaceRef.Name).To(Equal(name + "-lo2"))
					g.Expect(testProvider.NVE.Status.AnycastSourceInterfaceName).To(Equal(name + "-lo2"))
				}
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})
	})

	Context("When source interface is missing", func() {
		var (
			name      string
			nveKey    client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating device only (no interfaces)")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-missingif-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating an NVE object with a reference to a non-existent interface")
			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:        true,
					HostReachability:   "BGP",
					SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: name + "-lo-missing"},
					AdminState:         v1alpha1.AdminStateUp,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with WaitingForDependenciesReason", func() {
			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, cur)).To(Succeed())

				ready := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))

				cond := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})
	})

	Context("When AnycastSourceInterfaceRef is omitted", func() {
		var (
			name      string
			nveKey    client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-anycast-omit-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       name + "-lo0",
					Type:       v1alpha1.InterfaceTypeLoopback,
					AdminState: v1alpha1.AdminStateUp,
				},
			})).To(Succeed())

			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:        true,
					HostReachability:   "BGP",
					SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					AdminState:         v1alpha1.AdminStateUp,
					// AnycastSourceInterfaceRef: nil,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interface")
			ifObj := &v1alpha1.Interface{}
			ifKey := client.ObjectKey{Name: name + "-lo0", Namespace: metav1.NamespaceDefault}
			Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
			}).Should(BeTrue())

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should reconcile with nil anycast and empty status AnycastSourceInterfaceName", func() {
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).NotTo(BeNil())
				g.Expect(testProvider.NVE.Spec.AnycastSourceInterfaceRef).To(BeNil())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, cur)).To(Succeed())
				g.Expect(cur.Status.AnycastSourceInterfaceName).To(BeEmpty())
				cfg := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cfg).NotTo(BeNil())
				g.Expect(cfg.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})

	Context("When creating more than one NVE per device", func() {
		var (
			name      string
			nve1Key   client.ObjectKey
			nve2Key   client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-uniqueness-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			for _, ifName := range []string{name + "-lo0", name + "-lo1"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}

			nve1Key = client.ObjectKey{Name: name + "-nve1", Namespace: metav1.NamespaceDefault}
			nve2Key = client.ObjectKey{Name: name + "-nve2", Namespace: metav1.NamespaceDefault}

			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-nve1", Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:        true,
					HostReachability:   "BGP",
					SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					AdminState:         v1alpha1.AdminStateUp,
				},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-nve2", Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:        true,
					HostReachability:   "BGP",
					SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: name + "-lo1"},
					AdminState:         v1alpha1.AdminStateUp,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVEs")
			for _, nveKey := range []client.ObjectKey{nve1Key, nve2Key} {
				nveObj := &v1alpha1.NetworkVirtualizationEdge{}
				Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
				}).Should(BeTrue())
			}

			By("Cleaning up interfaces")
			for _, ifKey := range []client.ObjectKey{
				{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo1", Namespace: metav1.NamespaceDefault},
			} {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with NVEAlreadyExistsReason on the second NVE", func() {
			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nve2Key, cur)).To(Succeed())
				cond := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.NVEAlreadyExistsReason))
			}).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (non loopback type)", func() {
		var (
			name      string
			nveKey    client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-wrongiftype-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating interfaces with wrong type")
			for _, ifName := range []string{name + "-eth0", name + "-eth1"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypePhysical,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}

			By("Creating the custom resource for the Kind NetworkVirtualizationEdge")
			l2Prefix := v1alpha1.MustParsePrefix("234.0.0.0/8")
			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: name + "-eth0"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: name + "-eth1"},
					MulticastGroups:           &v1alpha1.MulticastGroups{L2: &l2Prefix},
					AdminState:                v1alpha1.AdminStateUp,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interfaces")
			for _, ifKey := range []client.ObjectKey{
				{Name: name + "-eth0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-eth1", Namespace: metav1.NamespaceDefault},
			} {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with InvalidInterfaceTypeReason", func() {
			Eventually(func(g Gomega) {
				current := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, current)).To(Succeed())
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.InvalidInterfaceTypeReason))
			}).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (cross-device reference)", func() {
		var (
			name      string
			nveKey    client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-crossdevice-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating a second device whose interfaces will be referenced cross-device")
			device2 := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-crossdevice-b-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device2)).To(Succeed())
			DeferCleanup(func() {
				d := &v1alpha1.Device{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: device2.Name, Namespace: metav1.NamespaceDefault}, d)).To(Succeed())
				Expect(k8sClient.Delete(ctx, d)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: device2.Name, Namespace: metav1.NamespaceDefault}, &v1alpha1.Device{}))
				}).Should(BeTrue())
			})

			By("Creating loopback interfaces on the second device")
			for _, ifName := range []string{name + "-lo0", name + "-lo1"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: device2.Name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}

			By("Creating the custom resource for the Kind NetworkVirtualizationEdge")
			l2Prefix := v1alpha1.MustParsePrefix("234.0.0.0/8")
			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: name + "-lo1"},
					MulticastGroups:           &v1alpha1.MulticastGroups{L2: &l2Prefix},
					AdminState:                v1alpha1.AdminStateUp,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interfaces")
			for _, ifKey := range []client.ObjectKey{
				{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo1", Namespace: metav1.NamespaceDefault},
			} {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False with CrossDeviceReferenceReason", func() {
			Eventually(func(g Gomega) {
				nve := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				cond := meta.FindStatusCondition(nve.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})

	Context("When using a non registered dependency for providerConfigRef", func() {
		var (
			name      string
			nveKey    client.ObjectKey
			deviceKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-nve-badproviderref-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			deviceKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
			nveKey = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating loopback interfaces")
			for _, ifName := range []string{name + "-lo0", name + "-lo1", name + "-lo2"} {
				Expect(k8sClient.Create(ctx, &v1alpha1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: metav1.NamespaceDefault},
					Spec: v1alpha1.InterfaceSpec{
						DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
						Name:       ifName,
						Type:       v1alpha1.InterfaceTypeLoopback,
						AdminState: v1alpha1.AdminStateUp,
					},
				})).To(Succeed())
			}

			By("Creating an NVE with an invalid providerConfigRef")
			Expect(k8sClient.Create(ctx, &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: name + "-lo0"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: name + "-lo1"},
					AdminState:                v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						Name:       name + "-lo2",
						Kind:       "Interface",
						APIVersion: "networking.metal.ironcore.dev/v1alpha1",
					},
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up NVE")
			nveObj := &v1alpha1.NetworkVirtualizationEdge{}
			Expect(k8sClient.Get(ctx, nveKey, nveObj)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nveObj)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
			}).Should(BeTrue())

			By("Cleaning up interfaces")
			for _, ifKey := range []client.ObjectKey{
				{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo1", Namespace: metav1.NamespaceDefault},
				{Name: name + "-lo2", Namespace: metav1.NamespaceDefault},
			} {
				ifObj := &v1alpha1.Interface{}
				Expect(k8sClient.Get(ctx, ifKey, ifObj)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
				}).Should(BeTrue())
			}

			By("Cleaning up Device")
			d := &v1alpha1.Device{}
			Expect(k8sClient.Get(ctx, deviceKey, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
			}).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
			}).Should(Succeed())
		})

		It("Should set Configured=False and an `IncompatibleProviderConfigRef` reason", func() {
			Eventually(func(g Gomega) {
				nve := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				cond := meta.FindStatusCondition(nve.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}).Should(Succeed())
		})
	})
})
