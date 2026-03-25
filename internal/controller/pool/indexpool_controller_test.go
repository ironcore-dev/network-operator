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

var _ = Describe("IndexPool Controller", func() {
	var pool *poolv1alpha1.IndexPool

	BeforeEach(func() {
		pool = &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "indexpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{
					corev1alpha1.MustParseIndexRange("1..10"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
	})

	It("Should successfully reconcile a IndexPool", func() {
		By("Updating the total and allocated status")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("10"))
			g.Expect(current.Status.Allocated).To(Equal("0"))
		}).Should(Succeed())
	})

	It("Should set Available=True when the pool has free capacity", func() {
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.AvailableCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.HasCapacityReason))
		}).Should(Succeed())
	})

	It("Should set Available=False when the pool is exhausted", func() {
		By("Exhausting the pool by filling all allocations in status")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("10"))
		}).Should(Succeed())

		By("Patching the pool status with full allocations")
		orig := pool.DeepCopy()
		allocations := make([]poolv1alpha1.IndexAllocation, 10)
		for i := range allocations {
			allocations[i] = poolv1alpha1.IndexAllocation{
				ClaimRef: corev1alpha1.LocalObjectReference{Name: "dummy"},
				Index:    uint64(i + 1),
			}
		}
		pool.Status.Allocations = allocations
		Expect(k8sClient.Status().Patch(ctx, pool, client.MergeFrom(orig))).To(Succeed())

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.AvailableCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ExhaustedReason))
		}).Should(Succeed())
	})
})
