// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NVE Controller", func() {
	Context("When reconciling a resource with cisco nx provider ref", func() {
		const name = "test-nve-with-prov-ref"
		const provConfigRefName = "test-nveconfig-prov-ref-refname"

		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
		pcKey := client.ObjectKey{Name: provConfigRefName, Namespace: metav1.NamespaceDefault}

		var (
			device    *v1alpha1.Device
			nve       *v1alpha1.NVE
			NVEConfig *nxv1alpha1.NVEConfig
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

			By("Ensuring Cisco NXOS config (Kind NVEConfig)")
			NVEConfig = &nxv1alpha1.NVEConfig{}
			if err := k8sClient.Get(ctx, pcKey, NVEConfig); errors.IsNotFound(err) {
				NVEConfig = &nxv1alpha1.NVEConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      provConfigRefName,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: nxv1alpha1.NVEConfigSpec{
						HoldDownTime:        300,
						AdvertiseVirtualMAC: true,
						InfraVLANs: []nxv1alpha1.VLANListItem{
							{ID: 100},
							{RangeMin: 300, RangeMax: 400},
						},
					},
				}
				Expect(k8sClient.Create(ctx, NVEConfig)).To(Succeed())
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
						SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "lo4"},
						AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo5"},
						MulticastGroups: &v1alpha1.MulticastGroups{
							L2: "234.1.1.1",
						},
						ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
							APIVersion: nxv1alpha1.GroupVersion.String(),
							Kind:       "NVEConfig",
							Name:       provConfigRefName,
						},
						AdminState: v1alpha1.AdminStateUp,
					},
				}
				Expect(k8sClient.Create(ctx, nve)).To(Succeed())
			}
		})

		It("Updating the contents of a referenced provider config ref should trigger a reconciliation", func() {
			testProvider.EnsureNVECalls = 0
			newHoldDownTime := uint16(400)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, pcKey, NVEConfig)).To(Succeed())
				NVEConfig.Spec.HoldDownTime = newHoldDownTime
				g.Expect(k8sClient.Update(ctx, NVEConfig)).To(Succeed())
			}).Should(Succeed())

			Eventually(func() int {
				return int(testProvider.EnsureNVECalls)
			}).Should(BeNumerically(">", 0))
		})

		It("Should not allow an additional NVEConfig for the same device", func() {
			secondNVEConfig := &nxv1alpha1.NVEConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-nveconfig",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: nxv1alpha1.NVEConfigSpec{
					HoldDownTime:        200,
					AdvertiseVirtualMAC: false,
				},
			}
			Eventually(func(g Gomega) {
				Expect(k8sClient.Create(ctx, secondNVEConfig)).To(Succeed())
			}).Should(Succeed())

			secondNVE := &v1alpha1.NVE{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nve-duplicate",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.NVESpec{
					DeviceRef:                 v1alpha1.LocalObjectReference{Name: name},
					SuppressARP:               false,
					HostReachability:          "BGP",
					SourceInterfaceRef:        v1alpha1.LocalObjectReference{Name: "lo4"},
					AnycastSourceInterfaceRef: &v1alpha1.LocalObjectReference{Name: "lo5"},
					ProviderConfigRef: &v1alpha1.TypedLocalObjectReference{
						APIVersion: nxv1alpha1.GroupVersion.String(),
						Kind:       "NVEConfig",
						Name:       "another-nveconfig",
					},
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, secondNVE)).To(Succeed())

			Eventually(func(g Gomega) {
				current := &v1alpha1.NVE{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-nve-duplicate", Namespace: metav1.NamespaceDefault}, current)).To(Succeed())
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(current.Status.Conditions).To(HaveLen(2))
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(ContainSubstring(nxv1alpha1.NVEConfigAlreadyExistsReason))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				err := k8sClient.Delete(ctx, secondNVE)
				if err != nil && !errors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
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
				g.Expect(testProvider.NVE.Spec.ProviderConfigRef).ToNot(BeNil(), "Provider NVE ProviderConfigRef should not be nil")
				g.Expect(testProvider.NVE.Spec.ProviderConfigRef.APIVersion).To(Equal(nxv1alpha1.GroupVersion.String()), "Provider NVE ProviderConfigRef APIVersion should be set")
				g.Expect(testProvider.NVE.Spec.ProviderConfigRef.Kind).To(Equal("NVEConfig"), "Provider NVE ProviderConfigRef Kind should be set")
				g.Expect(testProvider.NVE.Spec.ProviderConfigRef.Name).To(Equal(provConfigRefName), "Provider NVE ProviderConfigRef Name should be set")
			}).Should(Succeed())
		})
	})
})
