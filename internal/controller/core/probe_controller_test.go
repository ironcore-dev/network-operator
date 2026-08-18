// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("Probe Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-probe-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}
		})

		AfterEach(func() {
			By("Cleaning up the Probe resource")
			probe := &v1alpha1.Probe{}
			probe.Name = name
			probe.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, probe))).To(Succeed())

			By("Waiting for the Probe to be deleted")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				err := k8sClient.Get(ctx, key, resource)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should wait for its Device to become reachable", func() {
			testProvider.SetConnectError(errors.New("device unreachable"))
			DeferCleanup(func() { testProvider.SetConnectError(nil) })

			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, device)).To(Succeed())
				g.Expect(device.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReachableCondition),
					HaveField("Status", metav1.ConditionFalse),
				)))
			}).Should(Succeed())

			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypePing,
					Ping: &v1alpha1.PingProbe{
						Address: v1alpha1.MustParseAddr("192.0.2.1"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).To(BeNil())
				g.Expect(resource.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", v1alpha1.UnreachableReason),
				)))
			}).Should(Succeed())

			testProvider.SetConnectError(nil)
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
			}).Should(Succeed())
		})

		It("Should execute a Probe when its Device is paused", func() {
			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypePing,
					Ping: &v1alpha1.PingProbe{
						Address: v1alpha1.MustParseAddr("192.0.2.1"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, device)).To(Succeed())
				device.Spec.Paused = true
				g.Expect(k8sClient.Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
			}).Should(Succeed())
		})

		It("Should successfully reconcile a Ping Probe", func() {
			By("Creating the custom resource for the Kind Probe")
			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypePing,
					Ping: &v1alpha1.PingProbe{
						Address: v1alpha1.MustParseAddr("192.0.2.1"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
				g.Expect(resource.Status.NextRunTime).To(BeNil())
				g.Expect(resource.Status.Ping).NotTo(BeNil())
				g.Expect(resource.Status.Ping.Sent).To(Equal(int32(3)))
				g.Expect(resource.Status.Ping.Received).To(Equal(int32(3)))
				g.Expect(resource.Status.Ping.AvgTime).NotTo(BeNil())
				g.Expect(resource.Status.Ping.AvgTime.Duration).To(Equal(time.Millisecond))
				g.Expect(resource.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ProbeSuccessfulReason),
				)))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a MAC table entry Probe", func() {
			By("Creating the custom resource for the Kind Probe")
			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypeMACTableEntry,
					MACTableEntry: &v1alpha1.MACTableEntryProbe{
						MACAddress: "00:1a:2b:3c:4d:5e",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
				g.Expect(resource.Status.NextRunTime).To(BeNil())
				g.Expect(resource.Status.Ping).To(BeNil())
				g.Expect(resource.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ProbeSuccessfulReason),
				)))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a route presence Probe", func() {
			By("Creating the custom resource for the Kind Probe")
			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypeRoutePresence,
					RoutePresence: &v1alpha1.RoutePresenceProbe{
						Prefix: v1alpha1.MustParsePrefix("10.100.0.0/16"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
				g.Expect(resource.Status.NextRunTime).To(BeNil())
				g.Expect(resource.Status.Ping).To(BeNil())
				g.Expect(resource.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ProbeSuccessfulReason),
				)))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a VTEP peer connectivity Probe", func() {
			By("Creating the custom resource for the Kind Probe")
			resource := &v1alpha1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ProbeSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Type:      v1alpha1.ProbeTypeVTEPPeerConnectivity,
					VTEPPeerConnectivity: &v1alpha1.VTEPPeerConnectivityProbe{
						ExpectedPeers: []string{"192.0.2.10"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Probe{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.LastRunTime).NotTo(BeNil())
				g.Expect(resource.Status.NextRunTime).To(BeNil())
				g.Expect(resource.Status.Ping).To(BeNil())
				g.Expect(resource.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ProbeSuccessfulReason),
				)))
			}).Should(Succeed())
		})
	})
})
