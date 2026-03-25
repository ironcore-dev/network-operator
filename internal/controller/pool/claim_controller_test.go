// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

var _ = Describe("Claim Controller", func() {
	It("allocates an index from the referenced pool", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "index-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("100..101")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(controllerutil.ContainsFinalizer(currentClaim, poolv1alpha1.FinalizerName)).To(BeTrue())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Index).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("100"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal("1"))
			g.Expect(currentPool.Status.Allocations).To(HaveLen(1))
			g.Expect(currentPool.Status.Allocations[0].Index).To(Equal(uint64(100)))
		}).Should(Succeed())
	})

	It("allocates an ip address from the referenced pool", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ip-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{
					corev1alpha1.MustParsePrefix("10.0.0.0/30"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.IPAddress).To(HaveValue(Equal("10.0.0.0")))
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("10.0.0.0"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IPAddressPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal("1"))
			g.Expect(currentPool.Status.Allocations).To(HaveLen(1))
		}).Should(Succeed())
	})

	It("allocates a prefix from the referenced pool", func() {
		pool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "prefix-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes: []poolv1alpha1.IPPrefixPoolPrefix{
					{
						Prefix:       corev1alpha1.MustParsePrefix("10.1.0.0/24"),
						PrefixLength: 26,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPPrefixPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Prefix).To(HaveValue(HaveField("String()", Equal("10.1.0.0/26"))))
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("10.1.0.0/26"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal("1"))
			g.Expect(currentPool.Status.Allocations).To(HaveLen(1))
		}).Should(Succeed())
	})

	It("sets an invalid condition for unsupported pool references", func() {
		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: "unsupported.example.io/v1alpha1",
					Kind:       "UnsupportedPool",
					Name:       "unsupported-pool",
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.PoolRefInvalidReason))
		}).Should(Succeed())
	})

	It("sets a not found condition when the referenced pool does not exist", func() {
		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       "missing-pool",
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.PoolNotFoundReason))
			g.Expect(condition.Message).To(ContainSubstring("missing-pool"))
		}).Should(Succeed())
	})

	It("sets an exhausted condition when the pool has no allocations left", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "exhausted-index-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("42..42")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		firstClaim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, firstClaim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, firstClaim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(firstClaim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Index).NotTo(BeNil())
		}).Should(Succeed())

		secondClaim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, secondClaim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, secondClaim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			claim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secondClaim), claim)).To(Succeed())

			condition := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.PoolExhaustedReason))
			g.Expect(claim.Status.Allocation).To(BeNil())
		}).Should(Succeed())
	})

	It("releases allocations back to recycle pools on claim deletion", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "recycle-ip-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{
					corev1alpha1.MustParsePrefix("10.2.0.1/32"),
				},
				ReclaimPolicy: poolv1alpha1.ReclaimPolicyRecycle,
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.IPAddress).NotTo(BeNil())
		}).Should(Succeed())

		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IPAddressPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal("0"))
			g.Expect(currentPool.Status.Allocations).To(BeEmpty())
		}).Should(Succeed())
	})

	It("retains allocations on claim deletion when the pool reclaim policy is retain", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "retain-index-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges:        []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("500..500")},
				ReclaimPolicy: poolv1alpha1.ReclaimPolicyRetain,
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Index).NotTo(BeNil())
		}).Should(Succeed())

		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal("1"))
			g.Expect(currentPool.Status.Allocations).To(HaveLen(1))
			g.Expect(currentPool.Status.Allocations[0].Retained).To(BeTrue())
		}).Should(Succeed())
	})

	It("allocates the preferred index when the annotation is set", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "preferred-index-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("100..110")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
				Annotations: map[string]string{
					poolv1alpha1.PreferredValueAnnotation: "105",
				},
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Index).NotTo(BeNil())
			g.Expect(*currentClaim.Status.Allocation.Index).To(Equal(uint64(105)))
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("105"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})

	It("allocates the preferred IP address when the annotation is set", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "preferred-ip-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{
					corev1alpha1.MustParsePrefix("10.0.0.0/28"),
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
				Annotations: map[string]string{
					poolv1alpha1.PreferredValueAnnotation: "10.0.0.7",
				},
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPAddressPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.IPAddress).To(HaveValue(Equal("10.0.0.7")))
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("10.0.0.7"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})

	It("allocates the preferred prefix when the annotation is set", func() {
		pool := &poolv1alpha1.IPPrefixPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "preferred-prefix-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPPrefixPoolSpec{
				Prefixes: []poolv1alpha1.IPPrefixPoolPrefix{
					{
						Prefix:       corev1alpha1.MustParsePrefix("10.2.0.0/24"),
						PrefixLength: 26,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		claim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
				Annotations: map[string]string{
					poolv1alpha1.PreferredValueAnnotation: "10.2.0.128/26",
				},
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IPPrefixPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Allocation).NotTo(BeNil())
			g.Expect(currentClaim.Status.Allocation.Prefix).To(HaveValue(HaveField("String()", Equal("10.2.0.128/26"))))
			g.Expect(currentClaim.Status.Allocation.Value).To(Equal("10.2.0.128/26"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})

	It("sets PreferredValueUnavailable condition when the preferred value is taken", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "preferred-taken-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("100..100")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		firstClaim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, firstClaim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, firstClaim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(firstClaim), current)).To(Succeed())
			g.Expect(current.Status.Allocation).NotTo(BeNil())
			g.Expect(current.Status.Allocation.Index).NotTo(BeNil())
		}).Should(Succeed())

		secondClaim := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-",
				Namespace:    metav1.NamespaceDefault,
				Annotations: map[string]string{
					poolv1alpha1.PreferredValueAnnotation: "100",
				},
			},
			Spec: poolv1alpha1.ClaimSpec{
				PoolRef: corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       pool.Name,
				},
			},
		}
		Expect(k8sClient.Create(ctx, secondClaim)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, secondClaim))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			current := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secondClaim), current)).To(Succeed())

			condition := conditions.Get(current, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(poolv1alpha1.PreferredValueUnavailableReason))
			g.Expect(current.Status.Allocation).To(BeNil())
		}).Should(Succeed())
	})
})
