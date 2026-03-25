// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

var _ = Describe("IPPrefixPool Controller", func() {
	var pool *poolv1alpha1.IPPrefixPool

	BeforeEach(func() {
		pool = &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipprefixpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes: []poolv1alpha1.IPPrefixPoolPrefix{
					{
						Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
						PrefixLength: 28,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
	})

	It("Should successfully reconcile a IPPrefixPool", func() {
		By("Updating the total and allocated status")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("16"))
			g.Expect(current.Status.Allocated).To(Equal("0"))
		}).Should(Succeed())
	})

	It("Should set Available=True when the pool has free capacity", func() {
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.AvailableCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.HasCapacityReason))
		}).Should(Succeed())
	})

	It("Should set Available=False when the pool is exhausted", func() {
		By("Creating a pool with a single prefix so it can be exhausted")
		singlePool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipprefixpool-single-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes: []poolv1alpha1.IPPrefixPoolPrefix{
					{
						Prefix:       corev1alpha1.MustParsePrefix("10.9.0.0/30"),
						PrefixLength: 31,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, singlePool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, singlePool))).To(Succeed())
		})

		By("Patching the pool status with all prefixes allocated")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(singlePool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("2"))
		}).Should(Succeed())

		orig := singlePool.DeepCopy()
		singlePool.Status.Allocations = []poolv1alpha1.IPPrefixAllocation{
			{ClaimRef: corev1alpha1.LocalObjectReference{Name: "dummy-1"}, Prefix: corev1alpha1.MustParsePrefix("10.9.0.0/31")},
			{ClaimRef: corev1alpha1.LocalObjectReference{Name: "dummy-2"}, Prefix: corev1alpha1.MustParsePrefix("10.9.0.2/31")},
		}
		Expect(k8sClient.Status().Patch(ctx, singlePool, client.MergeFrom(orig))).To(Succeed())

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(singlePool), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.AvailableCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ExhaustedReason))
		}).Should(Succeed())
	})
})
