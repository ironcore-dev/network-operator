// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmi_test

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/txtar"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/test/gnmi/testutil"
)

const integrationNamespace = "default"

var _ = Describe("gNMI Integration", Ordered, Label("integration"), func() {
	// Create a secret for device credentials (required by DeviceReconciler)
	BeforeAll(func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "device-credentials",
				Namespace: integrationNamespace,
			},
			Type: corev1.SecretTypeBasicAuth,
			Data: map[string][]byte{
				corev1.BasicAuthUsernameKey: []byte("admin"),
				corev1.BasicAuthPasswordKey: []byte("admin"),
			},
		}
		err := integrationClient.Create(integrationCtx, secret)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	DescribeTable("Should reconcile resources via gNMI",
		func(ctx SpecContext, file string) {
			By("parsing txtar test file")
			path := filepath.Join("testdata", "cisco", "nxos", file)
			archive, err := txtar.ParseFile(path)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse test file %s", file)

			// Find sections in the txtar file
			var preloadState, expectedState string
			var manifests []txtar.File
			for _, f := range archive.Files {
				name := strings.TrimSpace(f.Name)
				switch name {
				case "state/preload":
					preloadState = strings.TrimSpace(string(f.Data))
				case "state/expect":
					expectedState = strings.TrimSpace(string(f.Data))
				default:
					// Everything else is a K8s manifest
					manifests = append(manifests, f)
				}
			}
			Expect(expectedState).NotTo(BeEmpty(), "No state/expect section found in %s", file)

			By("clearing gNMI server state")
			integrationServer.ClearState()

			// Preload initial state if present
			if preloadState != "" {
				By("preloading gNMI server state")
				integrationServer.SetState([]byte(preloadState))
				GinkgoWriter.Printf("Preloaded state: %s\n", preloadState)
			}

			By("creating Device pointing to gNMI server")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "device",
					Namespace: integrationNamespace,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: integrationGRPCAddr,
						SecretRef: &v1alpha1.SecretReference{
							Name: "device-credentials",
						},
					},
				},
			}
			err = integrationClient.Create(ctx, device)
			Expect(err).NotTo(HaveOccurred(), "Failed to create Device")

			// Wait for Device to be ready (Running phase)
			Eventually(func(g Gomega) {
				err := integrationClient.Get(ctx, client.ObjectKeyFromObject(device), device)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(device.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning), "Device not in Running phase")
			}).WithTimeout(testutil.LongTimeout).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())

			By("creating K8s resources from manifests")
			var createdObjects []*unstructured.Unstructured
			for _, manifest := range manifests {
				obj := &unstructured.Unstructured{}
				err = yaml.Unmarshal(manifest.Data, obj)
				Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal %s", manifest.Name)

				// Set namespace if not present
				if obj.GetNamespace() == "" {
					obj.SetNamespace(integrationNamespace)
				}

				GinkgoWriter.Printf("Creating %s/%s\n", obj.GetKind(), obj.GetName())
				err = integrationClient.Create(ctx, obj)
				Expect(err).NotTo(HaveOccurred(), "Failed to create %s", manifest.Name)
				createdObjects = append(createdObjects, obj)

				// Determine if we should wait for a condition
				// Operational resources: don't wait (they poll for state continuously)
				// Config-only resources: wait for Ready
				// VLAN: may not have conditions
				kind := obj.GetKind()
				switch kind {
				case "VLAN", "Interface", "BGP", "BGPPeer", "OSPF", "LLDP", "NetworkVirtualizationEdge", "DHCPRelay",
					"BGPConfig", "LLDPConfig": // Provider-specific config resources without their own controller
					// Skip condition wait for operational resources and VLANs
					// They either poll continuously or don't have conditions
					GinkgoWriter.Printf("Skipping condition wait for %s/%s (operational or no conditions)\n", kind, obj.GetName())
					continue
				}

			// Wait for a condition to be True
				// Prefer Configured (means config was pushed), fall back to Ready
				Eventually(func(g Gomega) {
					key := client.ObjectKeyFromObject(obj)
					refreshed := &unstructured.Unstructured{}
					refreshed.SetGroupVersionKind(obj.GroupVersionKind())

					g.Expect(integrationClient.Get(ctx, key, refreshed)).To(Succeed())

					conditions, found, err := unstructured.NestedSlice(refreshed.Object, "status", "conditions")
					g.Expect(err).NotTo(HaveOccurred())
					if !found {
						g.Expect(found).To(BeTrue(), "Resource %s/%s has no conditions", kind, obj.GetName())
						return
					}

					// Check for Configured=True first, then Ready=True
					var configuredTrue, readyTrue bool
					for _, c := range conditions {
						condMap, ok := c.(map[string]any)
						if !ok {
							continue
						}
						if condMap["type"] == "Configured" && condMap["status"] == "True" {
							configuredTrue = true
						}
						if condMap["type"] == "Ready" && condMap["status"] == "True" {
							readyTrue = true
						}
					}
					g.Expect(configuredTrue || readyTrue).To(BeTrue(), "Neither Configured nor Ready is True for %s/%s", kind, obj.GetName())
				}).WithTimeout(testutil.DefaultTimeout).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())
			}

			By("verifying gNMI server state")
			Eventually(func(g Gomega) {
				got, err := integrationServer.GetState()
				g.Expect(err).NotTo(HaveOccurred())

				err = testutil.CompareJSON(string(got), expectedState)
				if err != nil {
					GinkgoWriter.Printf("State not yet matching: %v\n", err)
				}
				g.Expect(err).NotTo(HaveOccurred(), "State mismatch")
			}).WithTimeout(10 * time.Second).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())

			By("cleaning up resources")
			// Delete resources in reverse order and wait for them to be gone
			for _, obj := range slices.Backward(createdObjects) {
				GinkgoWriter.Printf("Deleting %s/%s\n", obj.GetKind(), obj.GetName())
				Expect(client.IgnoreNotFound(integrationClient.Delete(ctx, obj))).To(Succeed())

				// Wait for the resource to be fully deleted
				key := client.ObjectKeyFromObject(obj)
				Eventually(func() bool {
					err := integrationClient.Get(ctx, key, obj)
					return client.IgnoreNotFound(err) == nil && err != nil
				}).WithTimeout(testutil.DefaultTimeout).WithPolling(testutil.DefaultPollingInterval).Should(BeTrue(),
					"Resource %s/%s was not deleted in time", obj.GetKind(), obj.GetName())
			}

			// Delete Device and wait for it to be gone
			Expect(client.IgnoreNotFound(integrationClient.Delete(ctx, device))).To(Succeed())
			Eventually(func() bool {
				err := integrationClient.Get(ctx, client.ObjectKeyFromObject(device), device)
				return client.IgnoreNotFound(err) == nil && err != nil
			}).WithTimeout(testutil.DefaultTimeout).WithPolling(testutil.DefaultPollingInterval).Should(BeTrue(),
				"Device was not deleted in time")
		},
		// Cisco NX-OS testdata entries
		Entry("ACL", "acl.txt"),
		Entry("Banner", "banner.txt"),
		Entry("BGP Peer", "bgp_bgppeer.txt"),
		Entry("DHCP Relay", "dhcprelay.txt"),
		Entry("DNS", "dns.txt"),
		Entry("EVPN Instance", "evpninstance.txt"),
		Entry("ISIS", "isis.txt"),
		Entry("LLDP", "lldp.txt"),
		Entry("Management Access", "managementaccess.txt"),
		Entry("NTP", "ntp.txt"),
		Entry("NVE", "nve.txt"),
		Entry("OSPF", "ospf.txt"),
		Entry("PIM", "pim.txt"),
		Entry("Routed VLAN", "routedvlan.txt"),
		Entry("Routing Policy Prefix Set", "routingpolicy_prefixset.txt"),
		Entry("SNMP", "snmp.txt"),
		Entry("Subinterface", "subinterface.txt"),
		Entry("Syslog", "syslog.txt"),
		Entry("VPC Domain", "vpcdomain.txt"),
		Entry("VRF", "vrf.txt"),
	)
})
