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

var _ = Describe("Index Controller", func() {
	It("sets Valid=True when the index value is within the pool range", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "idxpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{
					corev1alpha1.MustParseIndexRange("1..10"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

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
				Index: 5,
			},
		}
		Expect(k8sClient.Create(ctx, idx)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, idx))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.Index{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(idx), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueInRangeReason))
		}).Should(Succeed())
	})

	It("sets Valid=False when the index value is outside the pool range", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "idxpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{
					corev1alpha1.MustParseIndexRange("1..10"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

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
				Index: 99,
			},
		}
		Expect(k8sClient.Create(ctx, idx)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, idx))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.Index{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(idx), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueOutOfRangeReason))
		}).Should(Succeed())
	})
})
