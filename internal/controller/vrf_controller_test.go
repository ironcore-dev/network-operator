// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ = Describe("VRF Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-vrf"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		var (
			device *v1alpha1.Device
			vrf    *v1alpha1.VRF
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

			By("Creating the custom resource for the Kind VRF")
			vrf = &v1alpha1.VRF{}
			if err := k8sClient.Get(ctx, key, vrf); errors.IsNotFound(err) {
				vrf = &v1alpha1.VRF{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.VRFSpec{
						DeviceRef:          v1alpha1.LocalObjectReference{Name: name},
						Name:               "CC-ADMIN-TEST",
						VNI:                100,
						RouteDistinguisher: "127.0.0.1:30004",
						RouteTargets: []v1alpha1.RouteTarget{
							{
								Value:           "1.2.3.4:100",
								AddressFamilies: []v1alpha1.RouteTargetAF{v1alpha1.IPv4, v1alpha1.IPv6, v1alpha1.IPv4EVPN, v1alpha1.IPv6EVPN},
								Action:          v1alpha1.RouteTargetActionImport,
							},
							{
								Value:           "231231:100",
								AddressFamilies: []v1alpha1.RouteTargetAF{v1alpha1.IPv4, v1alpha1.IPv6},
								Action:          v1alpha1.RouteTargetActionExport,
							},
							{
								Value:           "65000:100",
								AddressFamilies: []v1alpha1.RouteTargetAF{v1alpha1.IPv4, v1alpha1.IPv6},
								Action:          v1alpha1.RouteTargetActionBoth,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, vrf)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, key, vrf)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NXOSVPC")
			Expect(k8sClient.Delete(ctx, vrf)).To(Succeed())

			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VRF).To(BeEmpty(), "Provider VPC should be empty")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vrf)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(vrf, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vrf)).To(Succeed())
				g.Expect(vrf.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vrf)).To(Succeed())
				g.Expect(vrf.OwnerReferences).To(HaveLen(1))
				g.Expect(vrf.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(vrf.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, vrf)).To(Succeed())
				g.Expect(vrf.Status.Conditions).To(HaveLen(1))
				g.Expect(vrf.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(vrf.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the VRF is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.VRF).ToNot(BeNil(), "Provider VRF should not be nil")
				if testProvider.VRF != nil {
					g.Expect(testProvider.VRF.Has("CC-ADMIN-TEST")).To(BeTrue(), "Provider should have VRF configured")
				}
			}).Should(Succeed())
		})
	})
})
