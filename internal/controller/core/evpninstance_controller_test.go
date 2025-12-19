// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("EVPNInstance Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-evi"
		const vni = 100010
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device := &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				resource := &v1alpha1.Device{
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up all EVPNInstance resources")
			Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.EVPNInstance{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

			By("Cleaning up test VLAN resource")
			vlan := &v1alpha1.VLAN{}
			if err := k8sClient.Get(ctx, key, vlan); err == nil {
				Expect(k8sClient.Delete(ctx, vlan)).To(Succeed())
			}

			device := &v1alpha1.Device{}
			err := k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the EVPNInstance is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.EVIs.Has(vni)).To(BeFalse(), "Provider shouldn't have VNI configured anymore")
			}).Should(Succeed())
		})

		It("Should successfully reconcile EVPNInstance with VLAN reference", func() {
			By("Creating a VLAN resource")
			vlan := &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					ID:         10,
					Name:       "vlan-10",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())

			By("Creating an EVPNInstance with complete configuration from sample")
			evi := &v1alpha1.EVPNInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EVPNInstanceSpec{
					DeviceRef:             v1alpha1.LocalObjectReference{Name: name},
					VNI:                   vni,
					Type:                  v1alpha1.EVPNInstanceTypeBridged,
					MulticastGroupAddress: "239.1.1.100",
					RouteDistinguisher:    "10.0.0.10:65000",
					RouteTargets: []v1alpha1.EVPNRouteTarget{
						{
							Value:  "65000:100010",
							Action: v1alpha1.RouteTargetActionBoth,
						},
					},
					VLANRef: &v1alpha1.LocalObjectReference{Name: name},
				},
			}
			Expect(k8sClient.Create(ctx, evi)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the VLAN is labeled with L2VNI label")
			Eventually(func(g Gomega) {
				vlanResource := &v1alpha1.VLAN{}
				g.Expect(k8sClient.Get(ctx, key, vlanResource)).To(Succeed())
				g.Expect(vlanResource.Labels).To(HaveKeyWithValue(v1alpha1.L2VNILabel, name))
			}).Should(Succeed())

			By("Verifying the VLAN status is updated with BridgedBy reference")
			Eventually(func(g Gomega) {
				vlanResource := &v1alpha1.VLAN{}
				g.Expect(k8sClient.Get(ctx, key, vlanResource)).To(Succeed())
				g.Expect(vlanResource.Status.BridgedBy).ToNot(BeNil())
				g.Expect(vlanResource.Status.BridgedBy.Name).To(Equal(name))
			}).Should(Succeed())

			By("Verifying the EVPNInstance is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.EVIs.Has(vni)).To(BeTrue(), "Provider should have VNI configured")
			}).Should(Succeed())
		})

		It("Should handle EVPNInstance referencing non-existent VLAN", func() {
			By("Creating an EVPNInstance referencing a non-existent VLAN")
			evi := &v1alpha1.EVPNInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EVPNInstanceSpec{
					DeviceRef:             v1alpha1.LocalObjectReference{Name: name},
					VNI:                   vni,
					Type:                  v1alpha1.EVPNInstanceTypeBridged,
					MulticastGroupAddress: "239.1.1.100",
					RouteDistinguisher:    "10.0.0.10:65000",
					RouteTargets: []v1alpha1.EVPNRouteTarget{
						{
							Value:  "65000:100010",
							Action: v1alpha1.RouteTargetActionBoth,
						},
					},
					VLANRef: &v1alpha1.LocalObjectReference{Name: "non-existent-vlan"},
				},
			}
			Expect(k8sClient.Create(ctx, evi)).To(Succeed())

			By("Verifying the controller sets VLAN not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.VLANNotFoundReason))
			}).Should(Succeed())
		})

		It("Should handle EVPNInstance referencing VLAN on different device", func() {
			By("Creating a VLAN on a different device")
			vlan := &v1alpha1.VLAN{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.VLANSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: "different-device"},
					ID:         10,
					Name:       "vlan-10",
					AdminState: v1alpha1.AdminStateUp,
				},
			}
			Expect(k8sClient.Create(ctx, vlan)).To(Succeed())

			By("Creating an EVPNInstance referencing the cross-device VLAN")
			evi := &v1alpha1.EVPNInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.EVPNInstanceSpec{
					DeviceRef:             v1alpha1.LocalObjectReference{Name: name},
					VNI:                   vni,
					Type:                  v1alpha1.EVPNInstanceTypeBridged,
					MulticastGroupAddress: "239.1.1.100",
					RouteDistinguisher:    "10.0.0.10:65000",
					RouteTargets: []v1alpha1.EVPNRouteTarget{
						{
							Value:  "65000:100010",
							Action: v1alpha1.RouteTargetActionBoth,
						},
					},
					VLANRef: &v1alpha1.LocalObjectReference{Name: name},
				},
			}
			Expect(k8sClient.Create(ctx, evi)).To(Succeed())

			By("Verifying the controller sets cross-device reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.EVPNInstance{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})
})
