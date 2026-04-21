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
			g.Expect(currentClaim.Status.Value).To(Equal("100"))
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())
			g.Expect(currentClaim.Status.AllocationRef.Kind).To(Equal("Index"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(1)))

			indices := &poolv1alpha1.IndexList{}
			g.Expect(k8sManager.GetClient().List(
				ctx, indices,
				client.InNamespace(metav1.NamespaceDefault),
				client.MatchingFields{poolRefIndexKey: pool.Name},
			)).To(Succeed())
			g.Expect(indices.Items).To(HaveLen(1))
			g.Expect(indices.Items[0].Spec.Index).To(Equal(int64(100)))
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
			g.Expect(currentClaim.Status.Value).To(Equal("10.0.0.0"))
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())
			g.Expect(currentClaim.Status.AllocationRef.Kind).To(Equal("IPAddress"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IPAddressPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(1)))

			ipaddresses := &poolv1alpha1.IPAddressList{}
			g.Expect(k8sManager.GetClient().List(
				ctx, ipaddresses,
				client.InNamespace(metav1.NamespaceDefault),
				client.MatchingFields{poolRefIndexKey: pool.Name},
			)).To(Succeed())
			g.Expect(ipaddresses.Items).To(HaveLen(1))
			g.Expect(ipaddresses.Items[0].Spec.Address.String()).To(Equal("10.0.0.0"))
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
			g.Expect(currentClaim.Status.Value).To(Equal("10.1.0.0/26"))
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())
			g.Expect(currentClaim.Status.AllocationRef.Kind).To(Equal("IPPrefix"))

			condition := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			currentPool := &poolv1alpha1.IPPrefixPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(1)))

			ipprefixes := &poolv1alpha1.IPPrefixList{}
			g.Expect(k8sManager.GetClient().List(
				ctx, ipprefixes,
				client.InNamespace(metav1.NamespaceDefault),
				client.MatchingFields{poolRefIndexKey: pool.Name},
			)).To(Succeed())
			g.Expect(ipprefixes.Items).To(HaveLen(1))
			g.Expect(ipprefixes.Items[0].Spec.Prefix.String()).To(Equal("10.1.0.0/26"))
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

			cond := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.PoolRefInvalidReason))
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

			cond := conditions.Get(currentClaim, poolv1alpha1.AllocatedCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.PoolNotFoundReason))
			g.Expect(cond.Message).To(ContainSubstring("missing-pool"))
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

		// Wait until first claim is allocated.
		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(firstClaim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Value).NotTo(BeEmpty())
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

			cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.PoolExhaustedReason))
			g.Expect(claim.Status.Value).To(BeEmpty())
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

		var allocName string
		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())
			allocName = currentClaim.Status.AllocationRef.Name
		}).Should(Succeed())

		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		// After deletion the IPAddress object should be gone (Recycle policy).
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{
				Name:      allocName,
				Namespace: metav1.NamespaceDefault,
			}, &poolv1alpha1.IPAddress{}))
		}).Should(BeTrue())

		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IPAddressPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(0)))
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

		var allocName string
		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())
			allocName = currentClaim.Status.AllocationRef.Name
		}).Should(Succeed())

		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claim))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		// After deletion the Index object should still exist (Retain policy) but have no claimRef.
		Eventually(func(g Gomega) {
			idx := &poolv1alpha1.Index{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      allocName,
				Namespace: metav1.NamespaceDefault,
			}, idx)).To(Succeed())
			g.Expect(idx.Spec.ClaimRef).To(BeNil())
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			// Index object still exists, so allocated count is still 1.
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(1)))
		}).Should(Succeed())
	})

	It("sets pool as owner of both the claim and the allocation", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "owner-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("1..10")},
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
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())

			// Pool owns the claim.
			g.Expect(currentClaim.OwnerReferences).To(ContainElement(SatisfyAll(
				HaveField("Kind", "IndexPool"),
				HaveField("Name", pool.Name),
			)))

			// Pool owns the allocation object.
			idx := &poolv1alpha1.Index{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      currentClaim.Status.AllocationRef.Name,
				Namespace: metav1.NamespaceDefault,
			}, idx)).To(Succeed())
			g.Expect(idx.OwnerReferences).To(ContainElement(SatisfyAll(
				HaveField("Kind", "IndexPool"),
				HaveField("Name", pool.Name),
			)))
		}).Should(Succeed())
	})

	It("transitions pool Available condition on allocation and release", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "avail-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("99..99")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		// Pool should initially be available.
		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			cond := conditions.Get(currentPool, poolv1alpha1.AvailableCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.HasCapacityReason))
		}).Should(Succeed())

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

		// After allocation, pool should be exhausted.
		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			cond := conditions.Get(currentPool, poolv1alpha1.AvailableCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.ExhaustedReason))
		}).Should(Succeed())

		// Delete the claim to release the slot.
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		// Pool should become available again.
		Eventually(func(g Gomega) {
			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			cond := conditions.Get(currentPool, poolv1alpha1.AvailableCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})

	It("stores the correct claim UID in the allocation claimRef", func() {
		pool := &poolv1alpha1.IPAddressPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "uid-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IPAddressPoolSpec{
				Prefixes: []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.5.0.0/30")},
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
			g.Expect(currentClaim.Status.AllocationRef).NotTo(BeNil())

			addr := &poolv1alpha1.IPAddress{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      currentClaim.Status.AllocationRef.Name,
				Namespace: metav1.NamespaceDefault,
			}, addr)).To(Succeed())
			g.Expect(addr.Spec.ClaimRef).NotTo(BeNil())
			g.Expect(addr.Spec.ClaimRef.Name).To(Equal(claim.Name))
			g.Expect(addr.Spec.ClaimRef.UID).To(Equal(claim.UID))
		}).Should(Succeed())
	})

	It("does not create duplicate allocations on re-reconciliation", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "idem-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("200..210")},
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

		// Wait for initial allocation.
		Eventually(func(g Gomega) {
			currentClaim := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
			g.Expect(currentClaim.Status.Value).To(Equal("200"))
		}).Should(Succeed())

		// Trigger re-reconciliation by patching a label on the claim.
		currentClaim := &poolv1alpha1.Claim{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), currentClaim)).To(Succeed())
		orig := currentClaim.DeepCopy()
		if currentClaim.Labels == nil {
			currentClaim.Labels = make(map[string]string)
		}
		currentClaim.Labels["trigger"] = "re-reconcile"
		Expect(k8sClient.Patch(ctx, currentClaim, client.MergeFrom(orig))).To(Succeed())

		// Verify only one allocation exists for this pool.
		Eventually(func(g Gomega) {
			indices := &poolv1alpha1.IndexList{}
			g.Expect(k8sManager.GetClient().List(
				ctx, indices,
				client.InNamespace(metav1.NamespaceDefault),
				client.MatchingFields{poolRefIndexKey: pool.Name},
			)).To(Succeed())
			g.Expect(indices.Items).To(HaveLen(1))

			currentPool := &poolv1alpha1.IndexPool{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pool), currentPool)).To(Succeed())
			g.Expect(currentPool.Status.Allocated).To(Equal(int64(1)))
		}).Should(Succeed())
	})

	It("allocates an exhausted claim after capacity is freed", func() {
		pool := &poolv1alpha1.IndexPool{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "retry-pool-",
				Namespace:    metav1.NamespaceDefault,
			},
			Spec: poolv1alpha1.IndexPoolSpec{
				Ranges: []corev1alpha1.IndexRange{corev1alpha1.MustParseIndexRange("77..77")},
			},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
		})

		// Claim A takes the only slot.
		claimA := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-a-",
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
		Expect(k8sClient.Create(ctx, claimA)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claimA))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			c := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claimA), c)).To(Succeed())
			g.Expect(c.Status.Value).To(Equal("77"))
		}).Should(Succeed())

		// Claim B hits exhaustion.
		claimB := &poolv1alpha1.Claim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "claim-b-",
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
		Expect(k8sClient.Create(ctx, claimB)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, claimB))).To(Succeed())
		})

		Eventually(func(g Gomega) {
			c := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claimB), c)).To(Succeed())
			cond := conditions.Get(c, poolv1alpha1.AllocatedCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(poolv1alpha1.PoolExhaustedReason))
		}).Should(Succeed())

		// Free the slot by deleting claim A.
		Expect(k8sClient.Delete(ctx, claimA)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claimA), &poolv1alpha1.Claim{}))
		}).Should(BeTrue())

		// Claim B should eventually succeed now that capacity is available.
		Eventually(func(g Gomega) {
			c := &poolv1alpha1.Claim{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claimB), c)).To(Succeed())
			g.Expect(c.Status.Value).To(Equal("77"))
			cond := conditions.Get(c, poolv1alpha1.AllocatedCondition)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}).Should(Succeed())
	})
})
