// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ = Describe("Interface Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-interface"
		key := types.NamespacedName{Name: name, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Interface")
			iface := &v1alpha1.Interface{}
			if err := k8sClient.Get(ctx, key, iface); errors.IsNotFound(err) {
				resource := &v1alpha1.Interface{
					TypeMeta: metav1.TypeMeta{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Interface",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.InterfaceSpec{
						Name:        name,
						AdminState:  "Up",
						Description: "Test",
						MTU:         9000,
						Type:        "Physical",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.Interface{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Interface")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				_, ok := testProvider.Items[name]
				g.Expect(ok).To(BeFalse(), "Resource should not exist in the provider")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Interface{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				item, ok := testProvider.Items[name]
				g.Expect(ok).To(BeTrue(), "Resource should exist in the provider")
				resource, ok := item.(*v1alpha1.Interface)
				g.Expect(ok).To(BeTrue(), "Resource should be of type Interface")
				g.Expect(resource.Spec.Name).To(Equal(name))
			}).Should(Succeed())
		})
	})
})
