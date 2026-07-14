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
				Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				AllocationPrefixLength: 28,
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
			g.Expect(current.Status.Allocated).To(Equal(int64(0)))
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
		By("Creating a pool with two /31 prefixes so it can be exhausted")
		singlePool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipprefixpool-single-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.9.0.0/30")},
				AllocationPrefixLength: 31,
			},
		}
		Expect(k8sClient.Create(ctx, singlePool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, singlePool))).To(Succeed())
		})

		By("Waiting for the pool total to be reconciled")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(singlePool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("2"))
		}).Should(Succeed())

		By("Creating IPPrefix objects to fill both slots")
		for _, cidr := range []string{"10.9.0.0/31", "10.9.0.2/31"} {
			pfx := &poolv1alpha1.IPPrefix{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "pfx-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: poolv1alpha1.IPPrefixSpec{
					PoolRef: corev1alpha1.TypedLocalObjectReference{
						APIVersion: poolv1alpha1.GroupVersion.String(),
						Kind:       "IPPrefixPool",
						Name:       singlePool.Name,
					},
					Prefix: corev1alpha1.MustParsePrefix(cidr),
				},
			}
			Expect(k8sClient.Create(ctx, pfx)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pfx))).To(Succeed())
			})
		}

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
