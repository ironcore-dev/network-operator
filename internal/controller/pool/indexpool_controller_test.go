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
			g.Expect(current.Status.Allocated).To(Equal(int64(0)))
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
		By("Waiting for the pool total to be reconciled")
		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), current)).To(Succeed())
			g.Expect(current.Status.Total).To(Equal("10"))
		}).Should(Succeed())

		By("Creating Index objects to fill all 10 slots")
		var createdIndices []*poolv1alpha1.Index
		for i := 1; i <= 10; i++ {
			idx := &poolv1alpha1.Index{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "idx-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: poolv1alpha1.IndexSpec{
					PoolRef: corev1alpha1.TypedLocalObjectReference{
						APIVersion: poolv1alpha1.GroupVersion.String(),
						Kind:       "IndexPool",
						Name:       pool.Name,
					},
					Index: int64(i),
				},
			}
			Expect(k8sClient.Create(ctx, idx)).To(Succeed())
			createdIndices = append(createdIndices, idx)
		}
		DeferCleanup(func() {
			for _, idx := range createdIndices {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, idx))).To(Succeed())
			}
		})

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
