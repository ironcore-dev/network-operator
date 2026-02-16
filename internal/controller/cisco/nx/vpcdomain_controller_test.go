// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nxv1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VPCDomain Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			vpcdomainName = "vpc1"
			deviceName    = "leaf1"
			poName        = "po1"
			vrfName       = "vrf1"
			physName      = "eth1-1"
		)
		var (
			deviceKey    = client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}
			vpcdomainKey = client.ObjectKey{Name: vpcdomainName, Namespace: metav1.NamespaceDefault}
			poKey        = client.ObjectKey{Name: poName, Namespace: metav1.NamespaceDefault}
			vrfKey       = client.ObjectKey{Name: vrfName, Namespace: metav1.NamespaceDefault}
			physKey      = client.ObjectKey{Name: physName, Namespace: metav1.NamespaceDefault}
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &corev1.Device{}
			if err := k8sClient.Get(ctx, deviceKey, device); errors.IsNotFound(err) {
				resource := &corev1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.DeviceSpec{
						Endpoint: corev1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind Interface (Physical)")
			phyIf := &corev1.Interface{}
			if err := k8sClient.Get(ctx, physKey, phyIf); errors.IsNotFound(err) {
				resource := &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      physName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceName},
						Name:       physName,
						Type:       corev1.InterfaceTypePhysical,
						AdminState: "Up",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind Interface (Aggregate)")
			aggIf := &corev1.Interface{}
			if err := k8sClient.Get(ctx, poKey, aggIf); errors.IsNotFound(err) {
				resource := &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{
						Name:      poName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceName},
						Name:       poName,
						Type:       corev1.InterfaceTypeAggregate,
						AdminState: "Up",
						Aggregation: &corev1.Aggregation{
							ControlProtocol: corev1.ControlProtocol{Mode: corev1.LACPModeActive},
							MemberInterfaceRefs: []corev1.LocalObjectReference{
								{Name: physName},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind VRF")
			vrf := &corev1.VRF{}
			if err := k8sClient.Get(ctx, vrfKey, vrf); errors.IsNotFound(err) {
				resource := &corev1.VRF{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vrfName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.VRFSpec{
						DeviceRef: corev1.LocalObjectReference{Name: deviceName},
						Name:      vrfName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind VPCDomain")
			vpc := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, vpc); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceName},
						AdminState:      "Up",
						DomainID:        2,
						RolePriority:    100,
						SystemPriority:  10,
						DelayRestoreSVI: 140,
						DelayRestoreVPC: 150,
						Peer: nxv1.Peer{
							AdminState:   "Up",
							InterfaceRef: corev1.LocalObjectReference{Name: poName},
							Switch:       nxv1.Enabled{Enabled: true},
							Gateway:      nxv1.Enabled{Enabled: true},
							KeepAlive: nxv1.KeepAlive{
								Source:      "10.114.235.155",
								Destination: "10.114.235.156",
								VrfRef:      &corev1.LocalObjectReference{Name: vrfName},
							},
							AutoRecovery: &nxv1.AutoRecovery{
								Enabled:     true,
								ReloadDelay: 360,
							},
						},
						FastConvergence: nxv1.Enabled{Enabled: true},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &nxv1.VPCDomain{}
			err := k8sClient.Get(ctx, vpcdomainKey, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VPCDomain")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &corev1.Device{}
			err = k8sClient.Get(ctx, deviceKey, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VPCDomain).To(BeNil(), "Provider VPCDomain should be nil")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
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
				g.Expect(resource.Labels).To(HaveKeyWithValue(corev1.DeviceLabel, deviceName))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(deviceName))
			}).Should(Succeed())

			// Set the peer-link port-channel interface status operational condition to True
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
				g.Expect(resource.Status.Conditions).To(HaveLen(3))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[2].Type).To(Equal(corev1.OperationalCondition))
				g.Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
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
			vpcdomainName  = "vpc1"
			deviceAName    = "leaf-a"
			deviceBName    = "leaf-b"
			vrfAName       = "leaf-a-vrf"
			vrfBName       = "leaf-b-vrf"
			physAName      = "leaf-a-eth-1-1"
			physBName      = "leaf-b-eth-1-1"
			lo0Name        = "leaf-a-lo0"
			aggAName       = "leaf-a-po1"
			aggBName       = "leaf-b-po1"
			missingIfName  = "vpc-missing-if"
			missingVrfName = "vpc-missing-vrf"
		)
		var (
			deviceAKey   = client.ObjectKey{Name: deviceAName, Namespace: metav1.NamespaceDefault}
			deviceBKey   = client.ObjectKey{Name: deviceBName, Namespace: metav1.NamespaceDefault}
			vpcdomainKey = client.ObjectKey{Name: vpcdomainName, Namespace: metav1.NamespaceDefault}
		)

		BeforeEach(func() {
			// Create devices A and B
			deviceA := &corev1.Device{}
			if err := k8sClient.Get(ctx, deviceAKey, deviceA); errors.IsNotFound(err) {
				resource := &corev1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceAName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.DeviceSpec{
						Endpoint: corev1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			deviceB := &corev1.Device{}
			if err := k8sClient.Get(ctx, deviceBKey, deviceB); errors.IsNotFound(err) {
				resource := &corev1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceBName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: corev1.DeviceSpec{
						Endpoint: corev1.Endpoint{
							Address: "192.168.10.3:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			// Create one physical interfaces on device A and B
			physA := &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      physAName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(physA), physA); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: physAName, Namespace: metav1.NamespaceDefault},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceAName},
						Name:       physAName,
						Type:       corev1.InterfaceTypePhysical,
						AdminState: "Up",
					},
				})).To(Succeed())
			}
			physB := &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      physBName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(physB), physB); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: physBName, Namespace: metav1.NamespaceDefault},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceBName},
						Name:       physBName,
						Type:       corev1.InterfaceTypePhysical,
						AdminState: "Up",
					},
				})).To(Succeed())
			}
			// Create aggregate interfaces on device A and B
			aggA := &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      aggAName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(aggA), aggA); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: aggAName, Namespace: metav1.NamespaceDefault},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceAName},
						Name:       aggAName,
						Type:       corev1.InterfaceTypeAggregate,
						AdminState: "Up",
						Aggregation: &corev1.Aggregation{
							ControlProtocol: corev1.ControlProtocol{Mode: corev1.LACPModeActive},
							MemberInterfaceRefs: []corev1.LocalObjectReference{
								{Name: physAName},
							},
						},
					},
				})).To(Succeed())
			}
			aggB := &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      aggBName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(aggB), aggB); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: aggBName, Namespace: metav1.NamespaceDefault},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceBName},
						Name:       aggBName,
						Type:       corev1.InterfaceTypeAggregate,
						AdminState: "Up",
						Aggregation: &corev1.Aggregation{
							ControlProtocol: corev1.ControlProtocol{Mode: corev1.LACPModeActive},
							MemberInterfaceRefs: []corev1.LocalObjectReference{
								{Name: physBName},
							},
						},
					},
				})).To(Succeed())
			}
			// VRFs on A and B
			vrfA := &corev1.VRF{ObjectMeta: metav1.ObjectMeta{Name: vrfAName, Namespace: metav1.NamespaceDefault}}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(vrfA), vrfA); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.VRF{
					ObjectMeta: metav1.ObjectMeta{Name: vrfAName, Namespace: metav1.NamespaceDefault},
					Spec:       corev1.VRFSpec{DeviceRef: corev1.LocalObjectReference{Name: deviceAName}, Name: vrfAName},
				})).To(Succeed())
			}
			vrfB := &corev1.VRF{ObjectMeta: metav1.ObjectMeta{Name: vrfBName, Namespace: metav1.NamespaceDefault}}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(vrfB), vrfB); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.VRF{
					ObjectMeta: metav1.ObjectMeta{Name: vrfBName, Namespace: metav1.NamespaceDefault},
					Spec:       corev1.VRFSpec{DeviceRef: corev1.LocalObjectReference{Name: deviceBName}, Name: vrfBName},
				})).To(Succeed())
			}
			// Loopback on A
			loA := &corev1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lo0Name,
					Namespace: metav1.NamespaceDefault,
				},
			}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(loA), loA); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Interface{
					ObjectMeta: metav1.ObjectMeta{Name: lo0Name, Namespace: metav1.NamespaceDefault},
					Spec: corev1.InterfaceSpec{
						DeviceRef:  corev1.LocalObjectReference{Name: deviceAName},
						Name:       lo0Name,
						Type:       corev1.InterfaceTypeLoopback,
						AdminState: "Up",
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &nxv1.VPCDomain{}
			err := k8sClient.Get(ctx, vpcdomainKey, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance VPCDomain")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &corev1.Device{}

			// Device A
			err = k8sClient.Get(ctx, deviceAKey, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device A")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			// Device B
			err = k8sClient.Get(ctx, deviceBKey, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device B")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VPCDomain).To(BeNil(), "Provider VPCDomain should be nil")
			}).Should(Succeed())
		})

		It("reports WaitingForDependencies when peer-link interface is missing", func() {
			v := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, v); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceAName},
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
								VrfRef:      &corev1.LocalObjectReference{Name: vrfAName},
							},
						},
						FastConvergence: nxv1.Enabled{Enabled: true},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})

		It("reports InvalidInterfaceType when peer-link reference is not aggregate or physical link", func() {
			v := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, v); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceAName},
						AdminState:      "Up",
						DomainID:        2,
						RolePriority:    100,
						SystemPriority:  10,
						DelayRestoreSVI: 140,
						DelayRestoreVPC: 150,
						Peer: nxv1.Peer{
							AdminState:   "Up",
							InterfaceRef: corev1.LocalObjectReference{Name: lo0Name},
							Switch:       nxv1.Enabled{Enabled: true},
							Gateway:      nxv1.Enabled{Enabled: true},
							KeepAlive: nxv1.KeepAlive{
								Source:      "10.114.235.155",
								Destination: "10.114.235.156",
								VrfRef:      &corev1.LocalObjectReference{Name: vrfAName},
							},
						},
						FastConvergence: nxv1.Enabled{Enabled: true},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.InvalidInterfaceTypeReason))
			}).Should(Succeed())
		})

		It("reports CrossDeviceReference when peer-link deviceRef mismatches VPCDomain deviceRef", func() {
			v := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, v); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceAName},
						AdminState:      "Up",
						DomainID:        2,
						RolePriority:    100,
						SystemPriority:  10,
						DelayRestoreSVI: 140,
						DelayRestoreVPC: 150,
						Peer: nxv1.Peer{
							AdminState:   "Up",
							InterfaceRef: corev1.LocalObjectReference{Name: aggBName},
							Switch:       nxv1.Enabled{Enabled: true},
							Gateway:      nxv1.Enabled{Enabled: true},
							KeepAlive: nxv1.KeepAlive{
								Source:      "10.114.235.155",
								Destination: "10.114.235.156",
								VrfRef:      &corev1.LocalObjectReference{Name: vrfAName},
							},
						},
						FastConvergence: nxv1.Enabled{Enabled: true},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})

		It("Sets status to WaitingForDependencies when KeepAlive VRF is missing", func() {
			v := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, v); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceAName},
						AdminState:      "Up",
						DomainID:        2,
						RolePriority:    100,
						SystemPriority:  10,
						DelayRestoreSVI: 140,
						DelayRestoreVPC: 150,
						Peer: nxv1.Peer{
							AdminState:   "Up",
							InterfaceRef: corev1.LocalObjectReference{Name: aggAName},
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.WaitingForDependenciesReason))
			}).Should(Succeed())
		})

		It("Sets status to CrossDeviceReference when KeepAlive VRF deviceRef mismatches VPCDomain deviceRef", func() {
			v := &nxv1.VPCDomain{}
			if err := k8sClient.Get(ctx, vpcdomainKey, v); errors.IsNotFound(err) {
				resource := &nxv1.VPCDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpcdomainName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1.VPCDomainSpec{
						DeviceRef:       corev1.LocalObjectReference{Name: deviceAName},
						AdminState:      "Up",
						DomainID:        2,
						RolePriority:    100,
						SystemPriority:  10,
						DelayRestoreSVI: 140,
						DelayRestoreVPC: 150,
						Peer: nxv1.Peer{
							AdminState:   "Up",
							InterfaceRef: corev1.LocalObjectReference{Name: aggAName},
							Switch:       nxv1.Enabled{Enabled: true},
							Gateway:      nxv1.Enabled{Enabled: true},
							KeepAlive: nxv1.KeepAlive{
								Source:      "10.114.235.155",
								Destination: "10.114.235.156",
								VrfRef:      &corev1.LocalObjectReference{Name: vrfBName},
							},
						},
						FastConvergence: nxv1.Enabled{Enabled: true},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &nxv1.VPCDomain{}
				g.Expect(k8sClient.Get(ctx, vpcdomainKey, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(corev1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(corev1.ConfiguredCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[1].Reason).To(Equal(corev1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})
})
