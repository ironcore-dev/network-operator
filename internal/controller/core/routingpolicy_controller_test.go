// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("RoutingPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-routingpolicy"
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
			rp := &v1alpha1.RoutingPolicy{}
			err := k8sClient.Get(ctx, key, rp)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the RoutingPolicy resource")
			Expect(k8sClient.Delete(ctx, rp)).To(Succeed())

			By("Cleaning up the PrefixSet resource")
			ps := &v1alpha1.PrefixSet{}
			if err := k8sClient.Get(ctx, key, ps); err == nil {
				Expect(k8sClient.Delete(ctx, ps)).To(Succeed())
			}

			device := &v1alpha1.Device{}
			err = k8sClient.Get(ctx, key, device)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the test Device resource")
			Expect(k8sClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())

			By("Verifying the RoutingPolicy is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeFalse(), "Provider shouldn't have RoutingPolicy configured anymore")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			rp := &v1alpha1.RoutingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.RejectRoute,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rp)).To(Succeed())

			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy with PrefixSet match condition and BGP actions", func() {
			By("Creating a PrefixSet resource")
			ps := &v1alpha1.PrefixSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.PrefixSetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "INTERNAL-NETWORKS",
					Entries: []v1alpha1.PrefixEntry{
						{
							Sequence: 10,
							Prefix:   v1alpha1.IPPrefix{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ps)).To(Succeed())

			By("Creating a RoutingPolicy with PrefixSet match condition and BGP actions")
			rp := &v1alpha1.RoutingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchPrefixSet: &v1alpha1.PrefixSetMatchCondition{
									PrefixSetRef: v1alpha1.LocalObjectReference{Name: name},
								},
							},
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetCommunity: &v1alpha1.SetCommunityAction{
										Communities: []string{"65137:100", "65137:200"},
									},
									SetExtCommunity: &v1alpha1.SetExtCommunityAction{
										Communities: []string{"65137:100"},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rp)).To(Succeed())

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should handle non-existing PrefixSet reference", func() {
			By("Creating a RoutingPolicy referencing non-existing PrefixSet")
			rp := &v1alpha1.RoutingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchPrefixSet: &v1alpha1.PrefixSetMatchCondition{
									PrefixSetRef: v1alpha1.LocalObjectReference{Name: "non-existing-prefixset"},
								},
							},
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rp)).To(Succeed())

			By("Verifying the controller sets PrefixSet not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.PrefixSetNotFoundReason))
			}).Should(Succeed())
		})

		It("Should handle PrefixSet on different device", func() {
			By("Creating a PrefixSet on a different device")
			ps := &v1alpha1.PrefixSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.PrefixSetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:      "INTERNAL-NETWORKS",
					Entries: []v1alpha1.PrefixEntry{
						{
							Sequence: 10,
							Prefix:   v1alpha1.IPPrefix{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ps)).To(Succeed())

			By("Creating a RoutingPolicy referencing the cross-device PrefixSet")
			rp := &v1alpha1.RoutingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchPrefixSet: &v1alpha1.PrefixSetMatchCondition{
									PrefixSetRef: v1alpha1.LocalObjectReference{Name: name},
								},
							},
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rp)).To(Succeed())

			By("Verifying the controller sets cross-device reference status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})
	})
})
