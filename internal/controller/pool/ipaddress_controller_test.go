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

var _ = Describe("IPAddress Controller", func() {
	It("sets Valid=True when the address is within the pool prefix", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipapool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{
					corev1alpha1.MustParsePrefix("10.0.0.0/24"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		ipa := &poolv1alpha1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipa-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       pool.Name,
				},
				Address: corev1alpha1.MustParseAddr("10.0.0.42"),
			},
		}
		Expect(k8sClient.Create(ctx, ipa)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ipa))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPAddress{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ipa), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueInRangeReason))
		}).Should(Succeed())
	})

	It("sets Valid=False when the address is outside the pool prefix", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipapool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{
					corev1alpha1.MustParsePrefix("10.0.0.0/24"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		ipa := &poolv1alpha1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ipa-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       pool.Name,
				},
				Address: corev1alpha1.MustParseAddr("192.168.1.1"),
			},
		}
		Expect(k8sClient.Create(ctx, ipa)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ipa))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.IPAddress{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ipa), current)).To(Succeed())
			condition := conditions.Get(current, poolv1alpha1.ValidCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.ValueOutOfRangeReason))
		}).Should(Succeed())
	})
})
