// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ResourceSet Controller", func() {
	Context("When reconciling a ResourceSet w/o matching Devices", func() {
		const name = "test-resource-set-no-devices"
		key := types.NamespacedName{Name: name, Namespace: metav1.NamespaceDefault}

		rs := &v1alpha1.ResourceSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ResourceSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1alpha1.ResourceSetSpec{
				Mode: v1alpha1.ResourceSetModeApplyOnce,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Resources: []v1alpha1.Resource{
					{
						Name:       "test-banner",
						Kind:       "Banner",
						APIVersion: v1alpha1.GroupVersion.String(),
						Template: runtime.RawExtension{
							Raw: []byte(`{
								"spec": {
									"message": {
										"inline": "Test Banner Message"
									}
								}
							}`),
						},
					},
				},
			},
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind ResourceSet")
			resource := &v1alpha1.ResourceSet{}
			if err := k8sClient.Get(ctx, key, resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, rs)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.ResourceSet{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ResourceSet")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.NoDevicesFoundReason))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})
	})

	Context("When reconciling a ResourceSet w/ invalid Resources", func() {
		const name = "test-resource-set-invalid"
		key := types.NamespacedName{Name: name, Namespace: metav1.NamespaceDefault}

		rs := &v1alpha1.ResourceSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ResourceSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1alpha1.ResourceSetSpec{
				Mode: v1alpha1.ResourceSetModeApplyOnce,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"test": name},
				},
				Resources: []v1alpha1.Resource{
					{
						Name:       "test-deployment",
						Kind:       "Pod",
						APIVersion: corev1.SchemeGroupVersion.String(),
						Template: runtime.RawExtension{
							Raw: []byte(`{
								"spec": {
									"containers": [
										{
											"name": "test-container",
											"image": "busybox:latest"
										}
									]
								}
							}`),
						},
					},
				},
			},
		}

		dev := &v1alpha1.Device{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "Device",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "test-device",
				Namespace: metav1.NamespaceDefault,
				Labels:    map[string]string{"test": name},
			},
			Spec: v1alpha1.DeviceSpec{
				Endpoint: &v1alpha1.Endpoint{
					Address: "127.0.0.1:9339",
				},
			},
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			var resource client.Object = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, dev)).To(Succeed())
			}

			By("Creating the custom resource for the Kind ResourceSet")
			resource = &v1alpha1.ResourceSet{}
			if err := k8sClient.Get(ctx, key, resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, rs)).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &v1alpha1.ResourceSet{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ResourceSet")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.NotReadyReason))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())
		})
	})

	Context("When reconciling a ResourceSet w/ mode ApplyOnce", func() {
		const name = "test-resource-set-apply-once"
		key := types.NamespacedName{Name: name, Namespace: metav1.NamespaceDefault}

		rs := &v1alpha1.ResourceSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ResourceSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1alpha1.ResourceSetSpec{
				Mode: v1alpha1.ResourceSetModeApplyOnce,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Resources: []v1alpha1.Resource{
					{
						Name:       "test-banner",
						Kind:       "Banner",
						APIVersion: v1alpha1.GroupVersion.String(),
						Template: runtime.RawExtension{
							Raw: []byte(`{
								"spec": {
									"message": {
										"inline": "Test Banner Message"
									}
								}
							}`),
						},
					},
				},
			},
		}

		dev := &v1alpha1.Device{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "Device",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "test-device",
				Namespace: metav1.NamespaceDefault,
				Labels:    map[string]string{"app": name},
			},
			Spec: v1alpha1.DeviceSpec{
				Endpoint: &v1alpha1.Endpoint{
					Address: "127.0.0.1:9339",
				},
			},
		}

		bannerKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-%s-%s", rs.Name, dev.Name, "test-banner"),
			Namespace: metav1.NamespaceDefault,
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			var resource client.Object = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, dev)).To(Succeed())
			}

			By("Creating the custom resource for the Kind ResourceSet")
			resource = &v1alpha1.ResourceSet{}
			if err := k8sClient.Get(ctx, key, resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, rs)).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &v1alpha1.ResourceSet{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ResourceSet")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Deleting the managed resource for the Kind Banner")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Banner{}
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, bannerKey, resource))).To(BeTrue(), "Banner resource should be deleted")
			}).Should(Succeed())

			resource = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Updating the managed resources status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.ManagedResources).To(HaveLen(1))
				g.Expect(resource.Status.ManagedResources[0].Name).To(Equal(bannerKey.Name))
				g.Expect(resource.Status.ManagedResources[0].Kind).To(Equal("Banner"))
				g.Expect(resource.Status.ManagedResources[0].APIVersion).To(Equal(v1alpha1.GroupVersion.String()))
				g.Expect(resource.Status.ManagedResources[0].Namespace).To(Equal(metav1.NamespaceDefault))
				g.Expect(resource.Status.ManagedResources[0].TargetName).To(Equal(dev.Name))
			}).Should(Succeed())

			By("Creating the custom resource for the Kind Banner")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Banner{}
				g.Expect(k8sClient.Get(ctx, bannerKey, resource)).To(Succeed())
				g.Expect(resource.Spec.Message.Inline).ToNot(BeNil())
				g.Expect(*resource.Spec.Message.Inline).To(Equal("Test Banner Message"))

				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, dev.Name))
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.OwnerLabel, rs.Name))

				g.Expect(controllerutil.HasControllerReference(resource)).To(BeTrue())
				g.Expect(controllerutil.HasOwnerReference(resource.GetOwnerReferences(), dev, k8sManager.GetScheme())).To(BeTrue())

				g.Expect(resource.OwnerReferences).To(HaveLen(2))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("ResourceSet"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(rs.Name))
				g.Expect(*resource.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())
				g.Expect(*resource.OwnerReferences[0].Controller).To(BeTrue())
				g.Expect(resource.OwnerReferences[1].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[1].Name).To(Equal(dev.Name))
				g.Expect(resource.OwnerReferences[1].BlockOwnerDeletion).To(BeNil())
				g.Expect(resource.OwnerReferences[1].Controller).To(BeNil())
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.AllResourcesReadyReason))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})

	Context("When reconciling a ResourceSet w/ mode Reconcile", func() {
		const name = "test-resource-set-reconcile"
		key := types.NamespacedName{Name: name, Namespace: metav1.NamespaceDefault}

		rs := &v1alpha1.ResourceSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ResourceSet",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1alpha1.ResourceSetSpec{
				Mode: v1alpha1.ResourceSetModeReconcile,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Resources: []v1alpha1.Resource{
					{
						Name:       "test-banner",
						Kind:       "Banner",
						APIVersion: v1alpha1.GroupVersion.String(),
						Template: runtime.RawExtension{
							Raw: []byte(`{
								"spec": {
									"message": {
										"inline": "Test Banner Message"
									}
								}
							}`),
						},
					},
				},
			},
		}

		dev := &v1alpha1.Device{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "Device",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "test-device",
				Namespace: metav1.NamespaceDefault,
				Labels:    map[string]string{"app": name},
			},
			Spec: v1alpha1.DeviceSpec{
				Endpoint: &v1alpha1.Endpoint{
					Address: "127.0.0.1:9339",
				},
			},
		}

		bannerKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-%s-%s", rs.Name, dev.Name, "test-banner"),
			Namespace: metav1.NamespaceDefault,
		}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			var resource client.Object = &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, dev)).To(Succeed())
			}

			By("Creating the custom resource for the Kind ResourceSet")
			resource = &v1alpha1.ResourceSet{}
			if err := k8sClient.Get(ctx, key, resource); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, rs)).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &v1alpha1.ResourceSet{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ResourceSet")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Deleting the managed resource for the Kind Banner")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Banner{}
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, bannerKey, resource))).To(BeTrue(), "Banner resource should be deleted")
			}).Should(Succeed())

			resource = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(dev), resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Updating the managed resources status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.ManagedResources).To(HaveLen(1))
				g.Expect(resource.Status.ManagedResources[0].Name).To(Equal(bannerKey.Name))
				g.Expect(resource.Status.ManagedResources[0].Kind).To(Equal("Banner"))
				g.Expect(resource.Status.ManagedResources[0].APIVersion).To(Equal(v1alpha1.GroupVersion.String()))
				g.Expect(resource.Status.ManagedResources[0].Namespace).To(Equal(metav1.NamespaceDefault))
				g.Expect(resource.Status.ManagedResources[0].TargetName).To(Equal(dev.Name))
			}).Should(Succeed())

			By("Creating the custom resource for the Kind Banner")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Banner{}
				g.Expect(k8sClient.Get(ctx, bannerKey, resource)).To(Succeed())
				g.Expect(resource.Spec.Message.Inline).ToNot(BeNil())
				g.Expect(*resource.Spec.Message.Inline).To(Equal("Test Banner Message"))

				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, dev.Name))
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.OwnerLabel, rs.Name))

				g.Expect(controllerutil.HasControllerReference(resource)).To(BeTrue())
				g.Expect(controllerutil.HasOwnerReference(resource.GetOwnerReferences(), dev, k8sManager.GetScheme())).To(BeTrue())

				g.Expect(resource.OwnerReferences).To(HaveLen(2))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("ResourceSet"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(rs.Name))
				g.Expect(*resource.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())
				g.Expect(*resource.OwnerReferences[0].Controller).To(BeTrue())
				g.Expect(resource.OwnerReferences[1].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[1].Name).To(Equal(dev.Name))
				g.Expect(resource.OwnerReferences[1].BlockOwnerDeletion).To(BeNil())
				g.Expect(resource.OwnerReferences[1].Controller).To(BeNil())
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ResourceSet{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Reason).To(Equal(v1alpha1.AllResourcesReadyReason))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})
})
