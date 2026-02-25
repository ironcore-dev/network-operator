// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

var _ = Describe("MacSec Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "macsec-test"
		const deviceName = "macsec-device"
		const intName = "macsec-interface"

		ctx = context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: metav1.NamespaceDefault,
		}

		macSecSecKey := types.NamespacedName{
			Name:      resourceName,
			Namespace: metav1.NamespaceDefault,
		}
		secretKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}
		deviceKey := client.ObjectKey{Name: deviceName, Namespace: metav1.NamespaceDefault}
		intKey := client.ObjectKey{Name: intName, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating the endpoint credentials as a Secret")
			deviceCreds := &corev1.Secret{}
			if err := k8sClient.Get(ctx, secretKey, deviceCreds); errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deviceName,
						Namespace: metav1.NamespaceDefault,
					},
					Data: map[string][]byte{
						corev1.BasicAuthUsernameKey: []byte("user"),
						corev1.BasicAuthPasswordKey: []byte("password"),
					},
					Type: corev1.SecretTypeBasicAuth,
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the device that will be referenced by the MacSec resource")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deviceName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
						SecretRef: &v1alpha1.SecretReference{
							Name: deviceName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Creating an Interface resource that will be referenced by the MacSec resource")
			intf := &v1alpha1.Interface{
				ObjectMeta: metav1.ObjectMeta{
					Name:      intName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:   v1alpha1.LocalObjectReference{Name: deviceName},
					Name:        intName,
					AdminState:  v1alpha1.AdminStateUp,
					Description: "Test",
					MTU:         9000,
					Type:        v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, intf)).To(Succeed())

			By("Creating the secret that will be referenced by the MacSec resource")
			macsecSecret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, macSecSecKey, macsecSecret); err != nil && errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: metav1.NamespaceDefault,
					},
					StringData: map[string]string{
						"lifetime":  "3600",
						"keyId":     "someID",
						"algorithm": "aes-256",
					},
					Type: corev1.SecretTypeOpaque,
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.MacSec{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance MacSec")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Cleanup the specific resource instance MacSec")

			By("Cleanup the specific resource instance Interface")
			intf := &v1alpha1.Interface{}
			err = k8sClient.Get(ctx, intKey, intf)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, intf)).To(Succeed())

			By("Cleanup the specific resource instance Device and Secret")
			device := &v1alpha1.Device{}
			err = k8sClient.Get(ctx, deviceKey, device)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, deviceKey, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind MacSec")
			macSec := &v1alpha1.MacSec{}
			err := k8sClient.Get(ctx, typeNamespacedName, macSec)
			if err != nil && errors.IsNotFound(err) {
				macSec = &v1alpha1.MacSec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1alpha1.MacSecSpec{
						DeviceRef:    v1alpha1.LocalObjectReference{Name: deviceName},
						InterfaceRef: v1alpha1.LocalObjectReference{Name: intName},
						Name:         "test-macsec",
						Description:  "Test MacSec resource",
						PreSharedKeyRef: []v1alpha1.LocalObjectReference{
							{
								Name: "macsec-test",
							},
						},
						Policy: &v1alpha1.MacSecPolicy{
							CipherSuite: "GCM-AES-256",
						},
					},
				}
			}
			Expect(k8sClient.Create(ctx, macSec)).To(Succeed())
			By("Reconciling Kind MacSec")
			locker, err := resourcelock.NewResourceLocker(
				k8sManager.GetClient(), metav1.NamespaceDefault,
				15*time.Second,
				10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &MacSecReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: func() provider.Provider { return testProvider },
				Locker:   locker,
			}

			// Add Finalizer
			//nolint:errcheck
			controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// Add status condition
			//nolint:errcheck
			controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// reconcile & update metadata
			//nolint:errcheck
			controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// update status
			//nolint:errcheck
			controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// update macsec to compare status
			Expect(k8sClient.Get(ctx, typeNamespacedName, macSec)).To(Succeed())
			Expect(macSec.Status.Conditions).To(HaveLen(1))
			Expect(macSec.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
			Expect(macSec.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
