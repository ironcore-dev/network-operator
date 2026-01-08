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

const (
	defaultTimeout   = 5 * time.Second
	defaultPoll      = 150 * time.Millisecond
	testEndpointAddr = "192.168.10.2:9339"
)

// Helpers
func ensureDevice(deviceKey client.ObjectKey, spec v1alpha1.DeviceSpec) {
	d := &v1alpha1.Device{}
	if err := k8sClient.Get(ctx, deviceKey, d); errors.IsNotFound(err) {
		d = &v1alpha1.Device{
			ObjectMeta: metav1.ObjectMeta{Name: deviceKey.Name, Namespace: deviceKey.Namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, d)).To(Succeed())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}
}

func ensureInterface(ns, deviceName, ifName string, ifType v1alpha1.InterfaceType) *v1alpha1.Interface {
	key := client.ObjectKey{Name: ifName, Namespace: ns}
	ifObj := &v1alpha1.Interface{}
	if err := k8sClient.Get(ctx, key, ifObj); errors.IsNotFound(err) {
		ifObj = &v1alpha1.Interface{
			ObjectMeta: metav1.ObjectMeta{Name: ifName, Namespace: ns},
			Spec: v1alpha1.InterfaceSpec{
				DeviceRef:  v1alpha1.LocalObjectReference{Name: deviceName},
				Name:       ifName,
				Type:       ifType,
				AdminState: v1alpha1.AdminStateUp,
			},
		}
		Expect(k8sClient.Create(ctx, ifObj)).To(Succeed())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}
	return ifObj
}

func ensureInterfaces(deviceName string, names []string, ifType v1alpha1.InterfaceType) {
	for _, name := range names {
		ensureInterface(metav1.NamespaceDefault, deviceName, name, ifType)
	}
}

func ensureNVE(nveKey client.ObjectKey, spec v1alpha1.NetworkVirtualizationEdgeSpec) *v1alpha1.NetworkVirtualizationEdge {
	n := &v1alpha1.NetworkVirtualizationEdge{}
	if err := k8sClient.Get(ctx, nveKey, n); errors.IsNotFound(err) {
		n = &v1alpha1.NetworkVirtualizationEdge{
			ObjectMeta: metav1.ObjectMeta{Name: nveKey.Name, Namespace: nveKey.Namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, n)).To(Succeed())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}
	return n
}

func cleanupNVEResources(nveKeys, interfaceKeys, deviceKeys []client.ObjectKey) {
	By("Cleaning up created resources")
	for _, nveKey := range nveKeys {
		nve := &v1alpha1.NetworkVirtualizationEdge{}
		Expect(k8sClient.Get(ctx, nveKey, nve)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, nve)).To(Succeed())
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, nveKey, &v1alpha1.NetworkVirtualizationEdge{}))
		}, defaultTimeout, defaultPoll).Should(BeTrue(), "NVE should be fully deleted")
	}

	for _, ifKey := range interfaceKeys {
		ifObj := &v1alpha1.Interface{}
		Expect(k8sClient.Get(ctx, ifKey, ifObj)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, ifObj)).To(Succeed())
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, ifKey, &v1alpha1.Interface{}))
		}, defaultTimeout, defaultPoll).Should(BeTrue(), "Interface should be fully deleted")
	}

	for _, deviceKey := range deviceKeys {
		device := &v1alpha1.Device{}
		Expect(k8sClient.Get(ctx, deviceKey, device)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, device)).To(Succeed())
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, deviceKey, &v1alpha1.Device{}))
		}, defaultTimeout, defaultPoll).Should(BeTrue(), "Device should be fully deleted")
	}
	By("Ensuring the resource is deleted from the provider")
	Eventually(func(g Gomega) {
		g.Expect(testProvider.NVE).To(BeNil(), "Provider NVE should be empty")
	}, defaultTimeout, defaultPoll).Should(Succeed())
}

var _ = Describe("NVE Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			deviceName = "test-nve-device"
			nveName    = "test-nve-nve"
			nsName     = metav1.NamespaceDefault
		)
		var (
			nve *v1alpha1.NetworkVirtualizationEdge
		)
		interfaceNames := []string{"lo0", "lo1"}

		nveKey := client.ObjectKey{Name: nveName, Namespace: nsName}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		interfaceKeys := make([]client.ObjectKey, len(interfaceNames))
		for i, ifName := range interfaceNames {
			interfaceKeys[i] = client.ObjectKey{Name: ifName, Namespace: nsName}
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Ensuring loopback interfaces exist")
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			By("Creating the custom resource for the Kind NVE")
			nve = ensureNVE(nveKey, v1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:                 v1alpha1.LocalObjectReference{Name: deviceName},
				SuppressARP:               true,
				HostReachability:          "BGP",
				SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
				AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceNames[1]},
				MulticastGroups:           &v1alpha1.MulticastGroups{L2: "234.0.0.1"},
				AdminState:                v1alpha1.AdminStateUp,
			})
		})

		AfterEach(func() {
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
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
				g.Expect(nve.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, deviceName))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				g.Expect(nve.OwnerReferences).To(HaveLen(1))
				g.Expect(nve.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(nve.OwnerReferences[0].Name).To(Equal(deviceName))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
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
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.SourceInterfaceRef.Name, Namespace: nsName}, primary)).To(Succeed())
				g.Expect(primary.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(primary.Spec.DeviceRef.Name).To(Equal(deviceName))

				anycast := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: nve.Spec.AnycastSourceInterfaceRef.Name, Namespace: nsName}, anycast)).To(Succeed())
				g.Expect(anycast.Spec.Type).To(Equal(v1alpha1.InterfaceTypeLoopback))
				g.Expect(anycast.Spec.DeviceRef.Name).To(Equal(deviceName))
				g.Expect(anycast.Name).NotTo(Equal(primary.Name)) // ensure different interfaces
			}).Should(Succeed())

			By("Verifying the controller sets valid reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, resource)).To(Succeed())
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
		const (
			deviceName = "test-nvewithrefupdates-device"
			nveName    = "test-nvewithrefupdates-nve"
			nsName     = metav1.NamespaceDefault
		)
		var (
			nve *v1alpha1.NetworkVirtualizationEdge
		)

		interfaceNames := []string{"lo10", "lo11", "lo12"}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey := client.ObjectKey{Name: nveName, Namespace: nsName}
		interfaceKeys := make([]client.ObjectKey, len(interfaceNames))
		for i, ifName := range interfaceNames {
			interfaceKeys[i] = client.ObjectKey{Name: ifName, Namespace: nsName}
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Ensuring loopback interfaces exist")
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			By("Creating the custom resource for the Kind NVE")
			nve = &v1alpha1.NetworkVirtualizationEdge{}
			if err := k8sClient.Get(ctx, nveKey, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NetworkVirtualizationEdge{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nveName,
						Namespace: nsName,
					},
					Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
						DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
						SuppressARP:        true,
						HostReachability:   "BGP",
						SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
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
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
		})

		It("Should reconcile when SourceInterfaceRef is changed", func() {
			By("Patching NVE: SourceInterfaceRef")
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.SourceInterfaceRef = v1alpha1.LocalObjectReference{Name: interfaceNames[1]}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Verifying reconciliation modifies provider and status")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).ToNot(BeNil())
				g.Expect(testProvider.NVE.Spec.SourceInterfaceRef.Name).To(Equal(interfaceNames[1]))
				g.Expect(testProvider.NVE.Status.SourceInterfaceName).To(Equal(interfaceNames[1]))
			}, defaultTimeout, defaultPoll).Should(Succeed())
		})

		It("Should reconcile when AnycastSourceInterfaceRef is added", func() {
			By("Patching NVE: AnycastSourceInterfaceRef")
			patch := client.MergeFrom(nve.DeepCopy())
			nve.Spec.AnycastSourceInterfaceRef = &v1alpha1.LocalObjectReference{Name: interfaceNames[2]}
			Expect(k8sClient.Patch(ctx, nve, patch)).To(Succeed())

			By("Verifying reconciliation modifies provider and status")
			Eventually(func(g Gomega) {
				if testProvider.NVE != nil {
					g.Expect(testProvider.NVE).ToNot(BeNil())
					g.Expect(testProvider.NVE.Spec.AnycastSourceInterfaceRef.Name).To(Equal(interfaceNames[2]))
					g.Expect(testProvider.NVE.Status.AnycastSourceInterfaceName).To(Equal(interfaceNames[2]))
				}
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})
	})

	Context("When source interface is missing", func() {
		const (
			deviceName = "test-nvemissingif-device"
			nveName    = "test-nvemissingif-nve"
			nsName     = metav1.NamespaceDefault
		)
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey := client.ObjectKey{Name: nveName, Namespace: nsName}

		BeforeEach(func() {
			By("Creating device only (no interfaces)")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Creating an NVE object with a reference to a non-existent interface")
			_ = ensureNVE(nveKey, v1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
				SuppressARP:        true,
				HostReachability:   "BGP",
				SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: "lo-missing"},
				AdminState:         v1alpha1.AdminStateUp,
			})
		})

		AfterEach(func() {
			cleanupNVEResources([]client.ObjectKey{nveKey}, nil, []client.ObjectKey{deviceKey})
		})

		It("Should set Configured=False with WaitingForDependenciesReason", func() {
			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, cur)).To(Succeed())
				cond := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.WaitingForDependenciesReason))
			}, defaultTimeout, defaultPoll).Should(Succeed())
		})
	})

	Context("When AnycastSourceInterfaceRef is omitted", func() {
		const (
			deviceName = "test-nve-anycast-omit-device"
			nveName    = "test-nve-anycast-omit"
			nsName     = metav1.NamespaceDefault
		)
		interfaceNames := []string{"lo30"}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey := client.ObjectKey{Name: nveName, Namespace: nsName}
		interfaceKeys := []client.ObjectKey{{Name: interfaceNames[0], Namespace: nsName}}

		BeforeEach(func() {
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			_ = ensureNVE(nveKey, v1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
				SuppressARP:        true,
				HostReachability:   "BGP",
				SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
				AdminState:         v1alpha1.AdminStateUp,
				// AnycastSourceInterfaceRef: nil,
			})
		})

		AfterEach(func() {
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
		})

		It("Should reconcile with nil anycast and empty status AnycastSourceInterfaceName", func() {
			Eventually(func(g Gomega) {
				g.Expect(testProvider.NVE).NotTo(BeNil())
				g.Expect(testProvider.NVE.Spec.AnycastSourceInterfaceRef).To(BeNil())
			}, defaultTimeout, defaultPoll).Should(Succeed())

			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, cur)).To(Succeed())
				g.Expect(cur.Status.AnycastSourceInterfaceName).To(BeEmpty())
				cfg := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cfg).NotTo(BeNil())
				g.Expect(cfg.Status).To(Equal(metav1.ConditionTrue))
			}, defaultTimeout, defaultPoll).Should(Succeed())
		})
	})

	Context("When creating more than one NVE per device", func() {
		const (
			deviceName = "test-nve-uniqueness-device"
			nve1Name   = "test-nve-uniqueness-1"
			nve2Name   = "test-nve-uniqueness-2"
			nsName     = metav1.NamespaceDefault
		)
		interfaceNames := []string{"lo40", "lo41"}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		nve1Key := client.ObjectKey{Name: nve1Name, Namespace: nsName}
		nve2Key := client.ObjectKey{Name: nve2Name, Namespace: nsName}
		interfaceKeys := []client.ObjectKey{
			{Name: interfaceNames[0], Namespace: nsName},
			{Name: interfaceNames[1], Namespace: nsName},
		}

		BeforeEach(func() {
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			_ = ensureNVE(nve1Key, v1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
				SuppressARP:        true,
				HostReachability:   "BGP",
				SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
				AdminState:         v1alpha1.AdminStateUp,
			})
			_ = ensureNVE(nve2Key, v1alpha1.NetworkVirtualizationEdgeSpec{
				DeviceRef:          v1alpha1.LocalObjectReference{Name: deviceName},
				SuppressARP:        true,
				HostReachability:   "BGP",
				SourceInterfaceRef: v1alpha1.LocalObjectReference{Name: interfaceNames[1]},
				AdminState:         v1alpha1.AdminStateUp,
			})
		})

		AfterEach(func() {
			cleanupNVEResources([]client.ObjectKey{nve1Key, nve2Key}, interfaceKeys, []client.ObjectKey{deviceKey})
		})

		It("Should set Configured=False with NVEAlreadyExistsReason on the second NVE", func() {
			Eventually(func(g Gomega) {
				cur := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nve2Key, cur)).To(Succeed())
				cond := meta.FindStatusCondition(cur.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.NVEAlreadyExistsReason))
			}, defaultTimeout, defaultPoll).Should(Succeed())
		})
	})

	Context("When using erroneous interface references (non loopback type)", func() {
		const (
			deviceName = "test-nvemisconfigurediftype-device"
			nveName    = "test-nvemisconfigurediftype-nve"
			nsName     = metav1.NamespaceDefault
		)
		var (
			nve *v1alpha1.NetworkVirtualizationEdge
		)

		interfaceNames := []string{"eth1", "eth2"}

		deviceKey := client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey := client.ObjectKey{Name: nveName, Namespace: nsName}
		interfaceKeys := make([]client.ObjectKey, len(interfaceNames))
		for i, ifName := range interfaceNames {
			interfaceKeys[i] = client.ObjectKey{Name: ifName, Namespace: nsName}
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Ensuring loopback interfaces with wrong type exist")
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypePhysical)

			By("Creating the custom resource for the Kind NetworkVirtualizationEdge")
			nve = &v1alpha1.NetworkVirtualizationEdge{}
			if err := k8sClient.Get(ctx, nveKey, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NetworkVirtualizationEdge{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nveName,
						Namespace: nsName,
					},
					Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
						DeviceRef:                 v1alpha1.LocalObjectReference{Name: deviceName},
						SuppressARP:               true,
						HostReachability:          "BGP",
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceNames[1]},
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
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
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
		const (
			deviceName  = "test-nvemisconfiguredcrossdevice-device"
			device2Name = "test-nvemisconfiguredcrossdevice-device2" // device for interface reference
			nveName     = "test-nvemisconfiguredcrossdevice-nve"
			nsName      = metav1.NamespaceDefault
		)

		var (
			nve               *v1alpha1.NetworkVirtualizationEdge
			deviceKey, nveKey client.ObjectKey
			interfaceKeys     []client.ObjectKey
		)

		interfaceNames := []string{"lo2", "lo3"}
		deviceKey = client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey = client.ObjectKey{Name: nveName, Namespace: nsName}
		interfaceKeys = make([]client.ObjectKey, len(interfaceNames))
		for i, ifName := range interfaceNames {
			interfaceKeys[i] = client.ObjectKey{Name: ifName, Namespace: nsName}
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Ensuring loopback interfaces with created on a different device")
			By("Ensuring loopback interfaces exist")
			ensureInterfaces(device2Name, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			By("Creating the custom resource for the Kind NetworkVirtualizationEdge")
			nve = &v1alpha1.NetworkVirtualizationEdge{}
			if err := k8sClient.Get(ctx, nveKey, nve); errors.IsNotFound(err) {
				nve = &v1alpha1.NetworkVirtualizationEdge{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nveName,
						Namespace: nsName,
					},
					Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
						DeviceRef:                 v1alpha1.LocalObjectReference{Name: deviceName},
						SuppressARP:               true,
						HostReachability:          "BGP",
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceNames[1]},
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
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
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
		const (
			deviceName = "test-nvemisconfigured-providerconfigref-device"
			nveName    = "test-nvemisconfigured-providerconfigref-nve"
			nsName     = metav1.NamespaceDefault
		)
		var (
			nve               *v1alpha1.NetworkVirtualizationEdge
			deviceKey, nveKey client.ObjectKey
		)

		interfaceNames := []string{"lo6", "lo7", "lo8"}
		deviceKey = client.ObjectKey{Name: deviceName, Namespace: nsName}
		nveKey = client.ObjectKey{Name: nveName, Namespace: nsName}
		interfaceKeys := make([]client.ObjectKey, len(interfaceNames))
		for i, ifName := range interfaceNames {
			interfaceKeys[i] = client.ObjectKey{Name: ifName, Namespace: nsName}
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			ensureDevice(deviceKey, v1alpha1.DeviceSpec{
				Endpoint: v1alpha1.Endpoint{Address: testEndpointAddr},
			})

			By("Ensuring loopback interfaces exist")
			ensureInterfaces(deviceName, interfaceNames, v1alpha1.InterfaceTypeLoopback)

			By("Ensuring an NVE with an invalid providerConfigRef")
			nve = &v1alpha1.NetworkVirtualizationEdge{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nveName,
					Namespace: nsName,
				},
				Spec: v1alpha1.NetworkVirtualizationEdgeSpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: deviceName},
					SuppressARP:               true,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: interfaceNames[0]},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: interfaceNames[1]},
					AdminState:                v1alpha1.AdminStateUp,
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						Name:       interfaceNames[2],
						Kind:       "Interface",
						APIVersion: "networking.metal.ironcore.dev/v1alpha1",
					}, // invalid provider config ref
				},
			}
			Expect(k8sClient.Create(ctx, nve)).To(Succeed())
		})

		AfterEach(func() {
			cleanupNVEResources([]client.ObjectKey{nveKey}, interfaceKeys, []client.ObjectKey{deviceKey})
		})

		It("Should set Configured=False and an `IncompatibleProviderConfigRef` reason", func() {
			Eventually(func(g Gomega) {
				nve := &v1alpha1.NetworkVirtualizationEdge{}
				g.Expect(k8sClient.Get(ctx, nveKey, nve)).To(Succeed())
				cond := meta.FindStatusCondition(nve.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.IncompatibleProviderConfigRef))
			}, defaultTimeout, defaultPoll).Should(Succeed())
		})
	})
})
