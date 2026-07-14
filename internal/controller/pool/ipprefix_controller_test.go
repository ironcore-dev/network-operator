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

var _ = Describe("IPPrefix Controller", func() {
	It("sets Valid=True when the prefix is within the pool", func() {
		pool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pfxpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				AllocationPrefixLength: 26,
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		pfx := &poolv1alpha1.IPPrefix{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pfx-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPPrefixPool",
					Name:       pool.Name,
				},
				Prefix: corev1alpha1.MustParsePrefix("10.0.0.64/26"),
			},
		}
		Expect(k8sClient.Create(ctx, pfx)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pfx))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefix{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pfx), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueInRangeReason))
		}).Should(Succeed())
	})

	It("sets Valid=False when the prefix is outside the pool", func() {
		pool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pfxpool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
				AllocationPrefixLength: 26,
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		pfx := &poolv1alpha1.IPPrefix{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pfx-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPPrefixPool",
					Name:       pool.Name,
				},
				Prefix: corev1alpha1.MustParsePrefix("192.168.1.0/26"),
			},
		}
		Expect(k8sClient.Create(ctx, pfx)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pfx))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPPrefix{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pfx), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueOutOfRangeReason))
		}).Should(Succeed())
	})
})
