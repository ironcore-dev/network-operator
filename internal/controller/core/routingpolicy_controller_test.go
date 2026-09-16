// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("RoutingPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating a Device resource for testing")
			device := &v1alpha1.Device{
				GenerateName: "test-routingpolicy-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the RoutingPolicy resource")
			rp := &v1alpha1.RoutingPolicy{}
			rp.Name = name
			rp.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, rp))).To(Succeed())

			By("Cleaning up the PrefixSet resource")
			ps := &v1alpha1.PrefixSet{}
			ps.Name = name
			ps.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ps))).To(Succeed())

			By("Cleaning up the CommunitySet resource")
			cs := &v1alpha1.CommunitySet{}
			cs.Name = name
			cs.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cs))).To(Succeed())

			By("Cleaning up the ExtCommunitySet resource")
			ecs := &v1alpha1.ExtCommunitySet{}
			ecs.Name = name
			ecs.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ecs))).To(Succeed())

			By("Verifying the RoutingPolicy is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeFalse(), "Provider shouldn't have RoutingPolicy configured anymore")
			}).Should(Succeed())

			By("Cleaning up the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy with PrefixSet match condition and BGP actions", func() {
			By("Creating a PrefixSet resource")
			ps := &v1alpha1.PrefixSet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should handle non-existing PrefixSet reference", func() {
			By("Creating a RoutingPolicy referencing non-existing PrefixSet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.PrefixSetNotFoundReason))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy with AS path actions", func() {
			By("Creating a RoutingPolicy with various AS path actions")
			asn65000 := intstr.FromInt32(65000)
			asn65001 := intstr.FromInt32(65001)
			asn65100 := intstr.FromInt32(65100)
			asnDotted := intstr.FromString("1.100")
			useLastAS := int32(5)

			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							// Prepend a specific ASN
							Sequence: 10,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetASPath: &v1alpha1.SetASPathAction{
										Prepend: &v1alpha1.SetASPathPrepend{
											ASNumber: &asn65000,
										},
									},
								},
							},
						},
						{
							// Prepend using last-as
							Sequence: 20,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetASPath: &v1alpha1.SetASPathAction{
										Prepend: &v1alpha1.SetASPathPrepend{
											UseLastAS: &useLastAS,
										},
									},
								},
							},
						},
						{
							// Replace private AS with a specific ASN
							Sequence: 30,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetASPath: &v1alpha1.SetASPathAction{
										Replace: &v1alpha1.SetASPathReplace{
											PrivateAS:   true,
											Replacement: asn65100,
										},
									},
								},
							},
						},
						{
							// Replace a specific ASN with another (using dotted notation)
							Sequence: 40,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetASPath: &v1alpha1.SetASPathAction{
										Replace: &v1alpha1.SetASPathReplace{
											ASNumber:    &asn65001,
											Replacement: asnDotted,
										},
									},
								},
							},
						},
						{
							// Set explicit AS path
							Sequence: 50,
							Actions: v1alpha1.PolicyActions{
								RouteDisposition: v1alpha1.AcceptRoute,
								BgpActions: &v1alpha1.BgpActions{
									SetASPath: &v1alpha1.SetASPathAction{
										ASNumber: &asn65000,
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should handle PrefixSet on different device", func() {
			By("Creating a PrefixSet on a different device")
			ps := &v1alpha1.PrefixSet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy with CommunitySet match condition", func() {
			By("Creating a CommunitySet resource")
			cs := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "CS-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, cs)).To(Succeed())

			By("Creating a RoutingPolicy with CommunitySet match condition using matchSetOptions ALL")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
									MatchSetOptions: v1alpha1.MatchSetOptionsAll,
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

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should handle non-existing CommunitySet reference", func() {
			By("Creating a RoutingPolicy referencing non-existing CommunitySet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: "non-existing-communityset"},
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

			By("Verifying the controller sets CommunitySet not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.CommunitySetNotFoundReason))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should handle CommunitySet on different device", func() {
			By("Creating a CommunitySet on a different device")
			cs := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: "different-device"},
					Name:      "CS-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, cs)).To(Succeed())

			By("Creating a RoutingPolicy referencing the cross-device CommunitySet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
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
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy with ExtCommunitySet match condition", func() {
			By("Creating an ExtCommunitySet resource")
			ecs := &v1alpha1.ExtCommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.ExtCommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "RT-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, ecs)).To(Succeed())

			By("Creating a RoutingPolicy with ExtCommunitySet match condition")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchExtCommunitySet: &v1alpha1.ExtCommunitySetMatchCondition{
									ExtCommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
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

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should handle non-existing ExtCommunitySet reference", func() {
			By("Creating a RoutingPolicy referencing non-existing ExtCommunitySet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchExtCommunitySet: &v1alpha1.ExtCommunitySetMatchCondition{
									ExtCommunitySetRef: v1alpha1.LocalObjectReference{Name: "non-existing-extcommunityset"},
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

			By("Verifying the controller sets ExtCommunitySet not found status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.ExtCommunitySetNotFoundReason))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a RoutingPolicy combining prefix, community and ext-community matches", func() {
			By("Creating a PrefixSet resource")
			ps := &v1alpha1.PrefixSet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.PrefixSetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "PL-DEVICE-V4",
					Entries: []v1alpha1.PrefixEntry{
						{
							Sequence: 10,
							Prefix:   v1alpha1.IPPrefix{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ps)).To(Succeed())

			By("Creating a CommunitySet resource")
			cs := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "CS-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, cs)).To(Succeed())

			By("Creating an ExtCommunitySet resource")
			ecs := &v1alpha1.ExtCommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.ExtCommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "RT-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, ecs)).To(Succeed())

			By("Creating a RoutingPolicy combining all three match conditions in one statement")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
								},
								MatchExtCommunitySet: &v1alpha1.ExtCommunitySetMatchCondition{
									ExtCommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
									MatchSetOptions:    v1alpha1.MatchSetOptionsAll,
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

			By("Verifying the controller sets successful status conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				g.Expect(resource.Status.Conditions[1].Type).To(Equal(v1alpha1.PausedCondition))
				g.Expect(resource.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should re-evaluate the RoutingPolicy when CommunitySet membership changes", func() {
			By("Creating a CommunitySet resource")
			cs := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "CS-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, cs)).To(Succeed())

			By("Creating a RoutingPolicy referencing the CommunitySet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
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

			By("Verifying the RoutingPolicy is configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should have RoutingPolicy configured")
			}).Should(Succeed())

			By("Recording the current generation observed by the controller")
			resource := &v1alpha1.RoutingPolicy{}
			Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
			var initialConditionTime metav1.Time
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).ToNot(BeEmpty())
				initialConditionTime = resource.Status.Conditions[0].LastTransitionTime
				g.Expect(initialConditionTime.IsZero()).To(BeFalse())
			}).Should(Succeed())

			By("Updating the CommunitySet membership")
			Eventually(func(g Gomega) {
				current := &v1alpha1.CommunitySet{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				current.Spec.Members = []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}, {Sequence: 20, Regex: "65137:200"}}
				g.Expect(k8sClient.Update(ctx, current)).To(Succeed())
			}).Should(Succeed())

			By("Verifying the RoutingPolicy remains configured after the membership change")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should still have RoutingPolicy configured")
			}).Should(Succeed())
		})

		It("Should reconverge when a CommunitySet match condition is removed from a statement", func() {
			By("Creating a CommunitySet resource")
			cs := &v1alpha1.CommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.CommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "CS-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, cs)).To(Succeed())

			By("Creating an ExtCommunitySet resource")
			ecs := &v1alpha1.ExtCommunitySet{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.ExtCommunitySetSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      "RT-BLUE",
					Members:   []v1alpha1.CommunityMember{{Sequence: 10, Regex: "65137:100"}},
				},
			}
			Expect(k8sClient.Create(ctx, ecs)).To(Succeed())

			By("Creating a RoutingPolicy matching both the CommunitySet and ExtCommunitySet")
			rp := &v1alpha1.RoutingPolicy{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.RoutingPolicySpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Name:      name,
					Statements: []v1alpha1.PolicyStatement{
						{
							Sequence: 10,
							Conditions: &v1alpha1.PolicyConditions{
								MatchCommunitySet: &v1alpha1.CommunitySetMatchCondition{
									CommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
								},
								MatchExtCommunitySet: &v1alpha1.ExtCommunitySetMatchCondition{
									ExtCommunitySetRef: v1alpha1.LocalObjectReference{Name: name},
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

			By("Verifying the RoutingPolicy reaches Ready with both conditions")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(2))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Removing the matchCommunitySet condition from the statement")
			Eventually(func(g Gomega) {
				current := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
				current.Spec.Statements[0].Conditions.MatchCommunitySet = nil
				g.Expect(k8sClient.Update(ctx, current)).To(Succeed())
			}).Should(Succeed())

			By("Deleting the CommunitySet to prove it is no longer referenced")
			Expect(k8sClient.Delete(ctx, cs)).To(Succeed())

			By("Verifying the RoutingPolicy stays Ready driven only by the remaining ExtCommunitySet match")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.RoutingPolicy{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Spec.Statements[0].Conditions.MatchCommunitySet).To(BeNil())
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the RoutingPolicy remains configured in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.RoutingPolicies.Has(name)).To(BeTrue(), "Provider should still have RoutingPolicy configured")
			}).Should(Succeed())
		})
	})
})
