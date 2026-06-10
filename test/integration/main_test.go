// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/txtar"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// resourceRegistry maps txtar file prefixes to GVKs for cleanup ordering.
// Resources are deleted in reverse order (last registered first).
var resourceRegistry = []schema.GroupVersionKind{
	v1alpha1.GroupVersion.WithKind("Interface"),
	v1alpha1.GroupVersion.WithKind("VLAN"),
	v1alpha1.GroupVersion.WithKind("VRF"),
	v1alpha1.GroupVersion.WithKind("NTP"),
	v1alpha1.GroupVersion.WithKind("DNS"),
	v1alpha1.GroupVersion.WithKind("LLDP"),
	v1alpha1.GroupVersion.WithKind("Banner"),
	v1alpha1.GroupVersion.WithKind("OSPF"),
	v1alpha1.GroupVersion.WithKind("PIM"),
	v1alpha1.GroupVersion.WithKind("NetworkVirtualizationEdge"),
	v1alpha1.GroupVersion.WithKind("EVPNInstance"),
	nxv1alpha1.GroupVersion.WithKind("InterfaceConfig"),
	nxv1alpha1.GroupVersion.WithKind("LLDPConfig"),
	v1alpha1.GroupVersion.WithKind("RoutingPolicy"),
	v1alpha1.GroupVersion.WithKind("PrefixSet"),
	// Add new resource types here as needed
}

var _ = Describe("Integration", func() {
	for _, providerCfg := range SupportedProviders {
		// Skip providers that don't have testdata
		testdataDir := filepath.Join("testdata", string(providerCfg.Name))
		if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
			continue
		}

		Describe(fmt.Sprintf("Provider: %s", providerCfg.Name), Ordered, func() {
			var ptc *ProviderTestContext
			var device *v1alpha1.Device
			var deviceName string

			BeforeAll(func() {
				By(fmt.Sprintf("setting up %s provider", providerCfg.Name))
				ptc = SetupProviderTest(providerCfg)
			})

			AfterAll(func() {
				By(fmt.Sprintf("tearing down %s provider", providerCfg.Name))
				TeardownProviderTest(ptc)
			})

			BeforeEach(func() {
				By("creating a test device with unique name")
				var err error
				device, err = CreateTestDevice(ctx, k8sClient, "")
				Expect(err).NotTo(HaveOccurred())
				deviceName = device.Name
			})

			AfterEach(func() {
				By("cleaning up resources")
				cleanupAllResources()

				// Delete the device and wait for it to be gone
				if device != nil {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
					Eventually(func(g Gomega) {
						err := k8sClient.Get(ctx, client.ObjectKeyFromObject(device), &v1alpha1.Device{})
						g.Expect(client.IgnoreNotFound(err)).To(Succeed())
						g.Expect(err).To(HaveOccurred(), "Device should be deleted")
					}).Should(Succeed())
				}

				// Clear gNMI state AFTER all resources are deleted and controllers have settled.
				By("clearing gNMI state for next test")
				time.Sleep(100 * time.Millisecond)
				ClearGNMIState()
			})

			// Discover test files for this provider
			testFiles, err := filepath.Glob(filepath.Join(testdataDir, "*.txt"))
			if err != nil {
				Fail(fmt.Sprintf("Failed to glob testdata: %v", err))
			}

			for _, testFile := range testFiles {
				testName := filepath.Base(testFile)
				testName = testName[:len(testName)-4] // remove .txt

				It("should reconcile "+testName, func(ctx SpecContext) {
					By("parsing testdata file")
					a, err := txtar.ParseFile(testFile)
					Expect(err).NotTo(HaveOccurred(), "Failed to parse test file: %s", testFile)
					Expect(len(a.Files)).To(BeNumerically(">=", 2), "Expected at least 2 files (resource(s) and state)")

					// Separate resources from state
					var stateData []byte
					var resources []txtar.File
					for _, f := range a.Files {
						if f.Name == "state" {
							stateData = f.Data
						} else {
							resources = append(resources, f)
						}
					}
					Expect(stateData).NotTo(BeEmpty(), "Expected '-- state --' section in testdata")
					Expect(resources).NotTo(BeEmpty(), "Expected at least one resource in testdata")

					By(fmt.Sprintf("creating %d resource(s) from testdata", len(resources)))
					for _, res := range resources {
						obj := createResource(ctx, res, deviceName)
						// Wait for each resource to be Ready before creating the next one.
						waitForReady(ctx, obj)
					}

					By("verifying gNMI state matches expected JSON")
					state, err := GetGNMIState()
					Expect(err).NotTo(HaveOccurred())

					err = CompareJSON(string(state), string(stateData))
					Expect(err).NotTo(HaveOccurred(), "gNMI state does not match expected JSON")
				})
			}
		})
	}
})

// createResource creates a K8s resource from txtar file data.
// The file name format is "kind/name" (e.g., "prefixset/my-prefixset").
// It substitutes "device" in deviceRef.name with the actual device name.
func createResource(ctx SpecContext, res txtar.File, deviceName string) client.Object {
	obj := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(res.Data, obj)).To(Succeed(), "Failed to unmarshal %s", res.Name)

	// Update deviceRef.name to use the actual device name
	if spec, ok := obj.Object["spec"].(map[string]any); ok {
		if deviceRef, ok := spec["deviceRef"].(map[string]any); ok {
			deviceRef["name"] = deviceName
		}
	}
	// Also update the device label
	labels := obj.GetLabels()
	if labels != nil {
		if _, ok := labels[v1alpha1.DeviceLabel]; ok {
			labels[v1alpha1.DeviceLabel] = deviceName
			obj.SetLabels(labels)
		}
	}

	Expect(k8sClient.Create(ctx, obj)).To(Succeed(), "Failed to create %s", res.Name)
	return obj
}

// waitForReady waits for a resource to have Configured=True condition.
// Skips resources that don't have status conditions (e.g., InterfaceConfig).
func waitForReady(ctx SpecContext, obj client.Object) {
	key := client.ObjectKeyFromObject(obj)
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Config-only resources don't have status conditions.
	// They are just referenced by other resources, not reconciled independently.
	switch gvk.Kind {
	case "InterfaceConfig", "LLDPConfig", "BGPConfig", "NVEConfig", "ManagementAccessConfig":
		return
	}

	Eventually(func(g Gomega) {
		// Fetch fresh copy
		fresh := &unstructured.Unstructured{}
		fresh.SetGroupVersionKind(gvk)
		g.Expect(k8sClient.Get(ctx, key, fresh)).To(Succeed())

		// Extract conditions from status
		conditions, found, err := unstructured.NestedSlice(fresh.Object, "status", "conditions")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(found).To(BeTrue(), "Conditions should be set for %s/%s", gvk.Kind, key.Name)
		g.Expect(conditions).NotTo(BeEmpty(), "Conditions should not be empty for %s/%s", gvk.Kind, key.Name)

		// Check for Configured=True (or Ready=True for resources without sub-conditions)
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			// Interface has sub-conditions; check Configured instead of Ready
			if cond["type"] == v1alpha1.ConfiguredCondition {
				g.Expect(cond["status"]).To(Equal(string(metav1.ConditionTrue)),
					"%s/%s should be Configured, got reason: %v, message: %v",
					gvk.Kind, key.Name, cond["reason"], cond["message"])
				return
			}
		}
		// Fallback: check Ready for resources without ConfiguredCondition (e.g., PrefixSet)
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if cond["type"] == v1alpha1.ReadyCondition {
				g.Expect(cond["status"]).To(Equal(string(metav1.ConditionTrue)),
					"%s/%s should be Ready, got reason: %v, message: %v",
					gvk.Kind, key.Name, cond["reason"], cond["message"])
			}
		}
	}).Should(Succeed())
}

// cleanupAllResources deletes all test resources in the correct order.
// It forcibly removes finalizers if deletion is blocked.
func cleanupAllResources() {
	for _, gvk := range resourceRegistry {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind + "List",
		})

		if err := k8sClient.List(ctx, list, client.InNamespace(metav1.NamespaceDefault)); err != nil {
			if meta.IsNoMatchError(err) {
				continue // CRD not installed, skip
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to list %s", gvk.Kind)
		}

		// First pass: remove ALL finalizers to prevent controller re-reconciliation
		for i := range list.Items {
			item := &list.Items[i]
			if len(item.GetFinalizers()) > 0 {
				item.SetFinalizers(nil)
				err := k8sClient.Update(ctx, item)
				if apierrors.IsConflict(err) {
					fresh := &unstructured.Unstructured{}
					fresh.SetGroupVersionKind(item.GroupVersionKind())
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), fresh)).To(Succeed())
					fresh.SetFinalizers(nil)
					Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}

		// Second pass: delete all resources
		for i := range list.Items {
			item := &list.Items[i]
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, item))).To(Succeed())
		}

		// Wait for deletion
		Eventually(func(g Gomega) {
			freshList := &unstructured.UnstructuredList{}
			freshList.SetGroupVersionKind(list.GetObjectKind().GroupVersionKind())
			g.Expect(k8sClient.List(ctx, freshList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
			g.Expect(freshList.Items).To(BeEmpty(), "%s resources should be deleted", gvk.Kind)
		}).Should(Succeed())
	}
}

// CompareJSON compares two JSON strings semantically (ignoring key order and array order).
func CompareJSON(got, want string) error {
	var gotData, wantData any
	if err := json.Unmarshal([]byte(got), &gotData); err != nil {
		return fmt.Errorf("failed to parse got JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(want), &wantData); err != nil {
		return fmt.Errorf("failed to parse want JSON: %w", err)
	}

	// Normalize both by sorting arrays recursively
	gotNorm := normalizeJSON(gotData)
	wantNorm := normalizeJSON(wantData)

	gotBytes, err := json.Marshal(gotNorm)
	if err != nil {
		return fmt.Errorf("failed to normalize got JSON: %w", err)
	}
	wantBytes, err := json.Marshal(wantNorm)
	if err != nil {
		return fmt.Errorf("failed to normalize want JSON: %w", err)
	}

	if !bytes.Equal(gotBytes, wantBytes) {
		return fmt.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", string(gotBytes), string(wantBytes))
	}
	return nil
}

// normalizeJSON recursively sorts arrays and returns a normalized structure.
func normalizeJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = normalizeJSON(v)
		}
		return result
	case []any:
		// Normalize each element first
		normalized := make([]any, len(val))
		for i, elem := range val {
			normalized[i] = normalizeJSON(elem)
		}
		// Sort the array by JSON representation
		sort.Slice(normalized, func(i, j int) bool {
			bi, _ := json.Marshal(normalized[i]) //nolint:errcheck // comparison only
			bj, _ := json.Marshal(normalized[j]) //nolint:errcheck // comparison only
			return string(bi) < string(bj)
		})
		return normalized
	default:
		return v
	}
}
