// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("MacSec Controller", func() {
	Context("When reconciling a resource", func() {
		const secretName = "macsec-secret"

		var (
			name   string
			intf   string
			key    client.ObjectKey
			device *v1alpha1.Device
		)

		BeforeEach(func() {
			By("Creating the Device that owns the MacSec resource")
			device = &v1alpha1.Device{
				GenerateName: "test-macsec-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the Interface referenced by the MacSec resource")
			intf = name
			iface := &v1alpha1.Interface{
				Name:      intf,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: name},
					Name:       intf,
					AdminState: v1alpha1.AdminStateUp,
					MTU:        9000,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, iface)).To(Succeed())

			By("Creating the pre-shared key Secret referenced by the MacSec resource")
			secret := &corev1.Secret{
				Name:      secretName,
				Namespace: metav1.NamespaceDefault,
				StringData: map[string]string{
					"lifetime":            "3600",
					"connectivityKeyName": "someID",
					"algorithm":           "aes-256",
				},
				Type: corev1.SecretTypeOpaque,
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the MacSec resource")
			macSec := &v1alpha1.MacSec{}
			macSec.Name = name
			macSec.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, macSec))).To(Succeed())

			By("Verifying the resource is removed from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(name).MacSec.Has("sven-test")).To(BeFalse())
			}).Should(Succeed())

			By("Waiting for the MacSec to be fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.MacSec{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the Interface resource")
			iface := &v1alpha1.Interface{}
			iface.Name = intf
			iface.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, iface))).To(Succeed())

			By("Cleaning up the Secret resource")
			secret := &corev1.Secret{}
			secret.Name = secretName
			secret.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, secret))).To(Succeed())

			By("Cleaning up the Device resource")
			dev := &v1alpha1.Device{}
			dev.Name = name
			dev.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dev))).To(Succeed())
		})

		It("Should successfully reconcile a MacSec resource", func() {
			By("Creating a MacSec resource")
			macSec := &v1alpha1.MacSec{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.MacSecSpec{
					DeviceRef:       v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:    v1alpha1.LocalObjectReference{Name: intf},
					Name:            "sven-test",
					Description:     "Test MacSec resource",
					PreSharedKeyRef: []v1alpha1.LocalObjectReference{{Name: secretName}},
					Policy:          &v1alpha1.MacSecPolicy{CipherSuite: "GCM-AES-256"},
				},
			}
			Expect(k8sClient.Create(ctx, macSec)).To(Succeed())

			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as an owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())

				ready := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))

				configured := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(configured).NotTo(BeNil())
				g.Expect(configured.Status).To(Equal(metav1.ConditionTrue))

				paused := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.PausedCondition)
				g.Expect(paused).NotTo(BeNil())
				g.Expect(paused.Status).To(Equal(metav1.ConditionFalse))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testDevices.StateFor(name).MacSec.Has("sven-test")).To(BeTrue())
			}).Should(Succeed())
		})

		It("Should report NotConfigured when the referenced interface is missing", func() {
			By("Creating a MacSec referencing a non-existent interface")
			macSec := &v1alpha1.MacSec{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.MacSecSpec{
					DeviceRef:       v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:    v1alpha1.LocalObjectReference{Name: "does-not-exist"},
					Name:            "sven-test",
					PreSharedKeyRef: []v1alpha1.LocalObjectReference{{Name: secretName}},
				},
			}
			Expect(k8sClient.Create(ctx, macSec)).To(Succeed())

			By("Reporting the Configured condition as InterfaceNotFound")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				configured := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(configured).NotTo(BeNil())
				g.Expect(configured.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(configured.Reason).To(Equal(v1alpha1.InterfaceNotFoundReason))
			}).Should(Succeed())
		})

		It("Should report NotConfigured when the interface belongs to another device", func() {
			By("Creating a second device and an interface on it")
			otherDevice := &v1alpha1.Device{
				GenerateName: "test-macsec-other-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.3:9339"},
					Provider: "test-provider",
				},
			}
			Expect(k8sClient.Create(ctx, otherDevice)).To(Succeed())

			otherIntf := &v1alpha1.Interface{
				GenerateName: "test-macsec-other-",
				Namespace:    metav1.NamespaceDefault,
				Spec: v1alpha1.InterfaceSpec{
					DeviceRef:  v1alpha1.LocalObjectReference{Name: otherDevice.Name},
					Name:       "other-interface",
					AdminState: v1alpha1.AdminStateUp,
					MTU:        9000,
					Type:       v1alpha1.InterfaceTypePhysical,
				},
			}
			Expect(k8sClient.Create(ctx, otherIntf)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherIntf))).To(Succeed())
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, otherDevice))).To(Succeed())
			})

			By("Creating a MacSec on the device referencing the other device's interface")
			macSec := &v1alpha1.MacSec{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.MacSecSpec{
					DeviceRef:       v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:    v1alpha1.LocalObjectReference{Name: otherIntf.Name},
					Name:            "sven-test",
					PreSharedKeyRef: []v1alpha1.LocalObjectReference{{Name: secretName}},
				},
			}
			Expect(k8sClient.Create(ctx, macSec)).To(Succeed())

			By("Reporting the Configured condition as CrossDeviceReference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				configured := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(configured).NotTo(BeNil())
				g.Expect(configured.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(configured.Reason).To(Equal(v1alpha1.CrossDeviceReferenceReason))
			}).Should(Succeed())
		})

		It("Should report NotConfigured when a pre-shared key secret is missing", func() {
			By("Creating a MacSec referencing a non-existent pre-shared key secret")
			macSec := &v1alpha1.MacSec{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Spec: v1alpha1.MacSecSpec{
					DeviceRef:       v1alpha1.LocalObjectReference{Name: name},
					InterfaceRef:    v1alpha1.LocalObjectReference{Name: intf},
					Name:            "sven-test",
					PreSharedKeyRef: []v1alpha1.LocalObjectReference{{Name: "missing-psk-secret"}},
				},
			}
			Expect(k8sClient.Create(ctx, macSec)).To(Succeed())

			By("Reporting the Configured condition as SecretNotFound")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.MacSec{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				configured := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ConfiguredCondition)
				g.Expect(configured).NotTo(BeNil())
				g.Expect(configured.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(configured.Reason).To(Equal(v1alpha1.SecretNotFoundReason))
			}).Should(Succeed())
		})
	})
})
