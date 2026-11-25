// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
)

var _ = Describe("ExternalPeering Controller", func() {
	Context("When reconciling a resource", func() {
		var key client.ObjectKey

		BeforeEach(func() {
			p := &overlayv1alpha1.ExternalPeering{}
			p.GenerateName = "test-externalpeering-"
			p.Namespace = metav1.NamespaceDefault
			p.Spec = overlayv1alpha1.ExternalPeeringSpec{}
			By("creating the custom resource for the Kind ExternalPeering")
			Expect(k8sClient.Create(ctx, p)).To(Succeed())

			key = client.ObjectKey{Name: p.Name, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			p := &overlayv1alpha1.ExternalPeering{}
			err := k8sClient.Get(ctx, key, p)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ExternalPeering")
			Expect(k8sClient.Delete(ctx, p)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &overlayv1alpha1.ExternalPeering{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, overlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())
		})
	})
})
