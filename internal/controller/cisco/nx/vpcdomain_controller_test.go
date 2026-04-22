// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nxv1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VPCDomain Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &corev1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-vpcdomain-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: corev1.DeviceSpec{
					Endpoint: corev1.Endpoint{
						Address: "192.168.10.2:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the custom resource for the Kind Interface (Physical)")
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-phys", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name},
					Name:       name + "-phys",
					Type:       corev1.InterfaceTypePhysical,
					AdminState: "Up",
				},
			})).To(Succeed())

			By("Creating the custom resource for the Kind Interface (Aggregate)")
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-po", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name},
					Name:       name + "-po",
					Type:       corev1.InterfaceTypeAggregate,
					AdminState: "Up",
					Aggregation: &corev1.Aggregation{
						ControlProtocol: corev1.ControlProtocol{Mode: corev1.LACPModeActive},
						MemberInterfaceRefs: []corev1.LocalObjectReference{
							{Name: name + "-phys"},
						},
					},
				},
			})).To(Succeed())

			By("Creating the custom resource for the Kind VRF")
			Expect(k8sClient.Create(ctx, &corev1.VRF{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-vrf", Namespace: metav1.NamespaceDefault},
				Spec: corev1.VRFSpec{
					DeviceRef: corev1.LocalObjectReference{Name: name},
					Name:      name + "-vrf",
				},
			})).To(Succeed())

			By("Creating the custom resource for the Kind VPCDomain")
			Expect(k8sClient.Create(ctx, &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-vpc", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: name + "-po"},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: name + "-vrf"},
						},
						AutoRecovery: &nxv1.AutoRecovery{
							Enabled:     true,
							ReloadDelay: 360,
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			vpcdomainKey := client.ObjectKey{Name: name + "-vpc", Namespace: metav1.NamespaceDefault}
			var resource client.Object = &nxv1.VPCDomain{}
			Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())

			By("Cleanup the specific resource instance VPCDomain")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &corev1.Device{}
			Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VPCDomain).To(BeNil(), "Provider VPCDomain should be nil")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			vpcdomainKey := client.ObjectKey{Name: name + "-vpc", Namespace: metav1.NamespaceDefault}
			poKey := client.ObjectKey{Name: name + "-po", Namespace: metav1.NamespaceDefault}

			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, corev1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(corev1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Setting the peer-link interface operational status")
			po := &corev1.Interface{}
			Expect(k8sClient.Get(ctx, poKey, po)).To(Succeed())
			meta.SetStatusCondition(&po.Status.Conditions, metav1.Condition{
				Type:    corev1.OperationalCondition,
				Status:  metav1.ConditionTrue,
				Reason:  corev1.OperationalReason,
				Message: "Interface is operational for testing",
			})
			Expect(k8sClient.Status().Update(ctx, po)).To(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VPCDomain).ToNot(BeNil(), "Provider VPCDomain should not be nil")
				if testProvider.VPCDomain != nil {
					g.Expect(testProvider.VPCDomain.Spec.DomainID).To(Equal(int16(2)))
				}
			}).Should(Succeed())
		})
	})

	Context("Dependency resolution", func() {
		const (
			missingIfName  = "vpc-missing-if"
			missingVrfName = "vpc-missing-vrf"
		)
		var (
			name         string
			vpcdomainKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating Device A")
			deviceA := &corev1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vpc-dep-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: corev1.DeviceSpec{Endpoint: corev1.Endpoint{Address: "192.168.10.2:9339"}},
			}
			Expect(k8sClient.Create(ctx, deviceA)).To(Succeed())
			name = deviceA.Name

			By("Creating Device B")
			Expect(k8sClient.Create(ctx, &corev1.Device{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-b", Namespace: metav1.NamespaceDefault},
				Spec:       corev1.DeviceSpec{Endpoint: corev1.Endpoint{Address: "192.168.10.3:9339"}},
			})).To(Succeed())

			By("Creating physical interfaces on Device A and B")
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-phys-a", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name},
					Name:       name + "-phys-a",
					Type:       corev1.InterfaceTypePhysical,
					AdminState: "Up",
				},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-phys-b", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name + "-b"},
					Name:       name + "-phys-b",
					Type:       corev1.InterfaceTypePhysical,
					AdminState: "Up",
				},
			})).To(Succeed())

			By("Creating aggregate interfaces on Device A and B")
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-po-a", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name},
					Name:       name + "-po-a",
					Type:       corev1.InterfaceTypeAggregate,
					AdminState: "Up",
					Aggregation: &corev1.Aggregation{
						ControlProtocol:     corev1.ControlProtocol{Mode: corev1.LACPModeActive},
						MemberInterfaceRefs: []corev1.LocalObjectReference{{Name: name + "-phys-a"}},
					},
				},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-po-b", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name + "-b"},
					Name:       name + "-po-b",
					Type:       corev1.InterfaceTypeAggregate,
					AdminState: "Up",
					Aggregation: &corev1.Aggregation{
						ControlProtocol:     corev1.ControlProtocol{Mode: corev1.LACPModeActive},
						MemberInterfaceRefs: []corev1.LocalObjectReference{{Name: name + "-phys-b"}},
					},
				},
			})).To(Succeed())

			By("Creating a loopback interface on Device A")
			Expect(k8sClient.Create(ctx, &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-lo0", Namespace: metav1.NamespaceDefault},
				Spec: corev1.InterfaceSpec{
					DeviceRef:  corev1.LocalObjectReference{Name: name},
					Name:       name + "-lo0",
					Type:       corev1.InterfaceTypeLoopback,
					AdminState: "Up",
				},
			})).To(Succeed())

			By("Creating VRFs on Device A and B")
			Expect(k8sClient.Create(ctx, &corev1.VRF{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-vrf-a", Namespace: metav1.NamespaceDefault},
				Spec:       corev1.VRFSpec{DeviceRef: corev1.LocalObjectReference{Name: name}, Name: name + "-vrf-a"},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.VRF{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-vrf-b", Namespace: metav1.NamespaceDefault},
				Spec:       corev1.VRFSpec{DeviceRef: corev1.LocalObjectReference{Name: name + "-b"}, Name: name + "-vrf-b"},
			})).To(Succeed())
		})

		AfterEach(func() {
			var resource client.Object = &nxv1.VPCDomain{}
			Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())

			By("Cleanup the VPCDomain")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Cleanup all Interface and VRF resources")
			Expect(k8sClient.DeleteAllOf(ctx, &corev1.Interface{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &corev1.VRF{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleanup Device A and B")
			Expect(k8sClient.Delete(ctx, &corev1.Device{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault}})).To(Succeed())
			Expect(k8sClient.Delete(ctx, &corev1.Device{ObjectMeta: metav1.ObjectMeta{Name: name + "-b", Namespace: metav1.NamespaceDefault}})).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VPCDomain).To(BeNil(), "Provider VPCDomain should be nil")
			}).Should(Succeed())
		})

		It("reports WaitingForDependencies when peer-link interface is missing", func() {
			vpc := &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "vpc-dep-", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: missingIfName},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: name + "-vrf-a"},
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, vpc)).To(Succeed())
			vpcdomainKey = client.ObjectKey{Name: vpc.Name, Namespace: metav1.NamespaceDefault}

			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.WaitingForDependenciesReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("reports InvalidInterfaceType when peer-link reference is not aggregate or physical", func() {
			vpc := &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "vpc-dep-", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: name + "-lo0"},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: name + "-vrf-a"},
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, vpc)).To(Succeed())
			vpcdomainKey = client.ObjectKey{Name: vpc.Name, Namespace: metav1.NamespaceDefault}

			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.InvalidInterfaceTypeReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("reports CrossDeviceReference when peer-link deviceRef mismatches VPCDomain deviceRef", func() {
			vpc := &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "vpc-dep-", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: name + "-po-b"},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: name + "-vrf-a"},
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, vpc)).To(Succeed())
			vpcdomainKey = client.ObjectKey{Name: vpc.Name, Namespace: metav1.NamespaceDefault}

			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("reports WaitingForDependencies when KeepAlive VRF is missing", func() {
			vpc := &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "vpc-dep-", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: name + "-po-a"},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: missingVrfName},
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, vpc)).To(Succeed())
			vpcdomainKey = client.ObjectKey{Name: vpc.Name, Namespace: metav1.NamespaceDefault}

			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.WaitingForDependenciesReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("reports CrossDeviceReference when KeepAlive VRF deviceRef mismatches VPCDomain deviceRef", func() {
			vpc := &nxv1.VPCDomain{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "vpc-dep-", Namespace: metav1.NamespaceDefault},
				Spec: nxv1.VPCDomainSpec{
					DeviceRef:       corev1.LocalObjectReference{Name: name},
					AdminState:      "Up",
					DomainID:        2,
					RolePriority:    100,
					SystemPriority:  10,
					DelayRestoreSVI: 140,
					DelayRestoreVPC: 150,
					Peer: nxv1.Peer{
						AdminState:   "Up",
						InterfaceRef: corev1.LocalObjectReference{Name: name + "-po-a"},
						Switch:       nxv1.Enabled{Enabled: true},
						Gateway:      nxv1.Enabled{Enabled: true},
						KeepAlive: nxv1.KeepAlive{
							Source:      "10.114.235.155",
							Destination: "10.114.235.156",
							VrfRef:      &corev1.LocalObjectReference{Name: name + "-vrf-b"},
						},
					},
					FastConvergence: nxv1.Enabled{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, vpc)).To(Succeed())
			vpcdomainKey = client.ObjectKey{Name: vpc.Name, Namespace: metav1.NamespaceDefault}

			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(4))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionUnknown))
				g.Expect(resource.Status.Conditions[3].Type).To(Equal(corev1.PausedCondition))
				g.Expect(resource.Status.Conditions[3].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})
	})
})
