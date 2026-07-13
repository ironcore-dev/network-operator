// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmi_test

import (
	"path/filepath"
	"slices"
	"strings"

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

				// Determine wait condition from resource type
				// Config-only resources: wait for Ready (which reflects Configured internally)
				// Operational resources: wait for Configured
				var condition string
				kind := obj.GetKind()
				switch kind {
				case "VLAN":
					// VLANs may not have conditions in openconfig mode
					continue
				case "Interface", "BGP", "BGPPeer", "OSPF", "LLDP", "NetworkVirtualizationEdge", "DHCPRelay":
					// Resources with operational state
					condition = "Configured"
				default:
					// Config-only resources (Banner, ACL, DNS, NTP, Syslog, etc.)
					condition = "Ready"
				}

				// Wait for condition
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

					// Find matching condition
					var conditionFound bool
					for _, c := range conditions {
						condMap, ok := c.(map[string]interface{})
						if !ok {
							continue
						}
						if condMap["type"] == condition && condMap["status"] == "True" {
							conditionFound = true
							break
						}
					}
					g.Expect(conditionFound).To(BeTrue(), "Condition %s not True for %s/%s", condition, kind, obj.GetName())
				}).WithTimeout(testutil.VeryLongTimeout).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())
			}

			By("verifying gNMI server state")
			Eventually(func(g Gomega) {
				got, err := integrationServer.GetState()
				g.Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Printf("Actual state: %s\n", string(got))
				GinkgoWriter.Printf("Expected state: %s\n", expectedState)

				err = testutil.CompareJSON(string(got), expectedState)
				g.Expect(err).NotTo(HaveOccurred(), "State mismatch")
			}).WithTimeout(testutil.LongTimeout).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())

			By("cleaning up resources")
			// Delete resources in reverse order
			for _, obj := range slices.Backward(createdObjects) {
				GinkgoWriter.Printf("Deleting %s/%s\n", obj.GetKind(), obj.GetName())
				err := integrationClient.Delete(ctx, obj)
				Expect(client.IgnoreNotFound(err)).To(Succeed())
			}

			// Delete Device
			err = integrationClient.Delete(ctx, device, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(client.IgnoreNotFound(err)).To(Succeed())

			// Wait for Device deletion
			Eventually(func(g Gomega) {
				err := integrationClient.Get(ctx, client.ObjectKeyFromObject(device), device)
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
				g.Expect(err).To(HaveOccurred(), "Device should be deleted")
			}).WithTimeout(testutil.LongTimeout).WithPolling(testutil.DefaultPollingInterval).Should(Succeed())
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
