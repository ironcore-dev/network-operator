// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
)

var _ = Describe("Network Controller", func() {
	Context("When reconciling a resource", func() {
		var key client.ObjectKey

		BeforeEach(func() {
			resource := &overlayv1alpha1.Network{
				GenerateName: "test-network-",
				Namespace:    metav1.NamespaceDefault,
			}
			By("creating the custom resource for the Kind Network")
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			key = client.ObjectKey{Name: resource.Name, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			resource := &overlayv1alpha1.Network{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Network")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, overlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller updates the status conditions")
			Eventually(func(g Gomega) {
				resource := &overlayv1alpha1.Network{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})
})
