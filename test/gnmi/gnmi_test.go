// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmitest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/txtar"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("gNMI requests tests", func() {
	// Tree construction: discover test files to generate It() nodes.
	// Provider resolution happens in BeforeSuite.
	envProvider := os.Getenv(ProviderEnvVar)

	testdataDir := filepath.Join("testdata", envProvider)
	testFiles, err := filepath.Glob(filepath.Join(testdataDir, "*.txtar"))
	if err != nil {
		Fail(fmt.Sprintf("failed to glob testdata: %v", err))
	}
	if len(testFiles) == 0 {
		return
	}

	// Tests run in parallel - each test gets its own namespace
	Describe("Provider: "+envProvider, func() {
		var testNamespace string

		BeforeEach(func(ctx SpecContext) {
			By("creating dedicated test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "reconcile-gnmi-test-",
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			testNamespace = ns.Name
		})

		AfterEach(func(ctx SpecContext) {
			By("deleting the test namespace")
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}))).To(Succeed())
		})

		// Generate individual It nodes for each test file
		for _, testFile := range testFiles {
			testName := filepath.Base(testFile)
			testName = testName[:len(testName)-6] // remove .txtar

			It("It should handle the configuration in test file "+testName, func(ctx SpecContext) {
				By("parsing testdata file")
				a, err := txtar.ParseFile(testFile)
				Expect(err).NotTo(HaveOccurred(), "Failed to parse test file: %s", testFile)
				Expect(len(a.Files)).To(BeNumerically(">=", 2), "Expected at least 2 files (resource(s) and state)")

				var statePre, statePost, stateDelete []byte
				var resources []txtar.File
				for _, f := range a.Files {
					switch f.Name {
					case "state/expect":
						statePost = f.Data
					case "state/preload":
						statePre = f.Data
					case "state/delete":
						stateDelete = f.Data
					default:
						resources = append(resources, f)
					}
				}
				Expect(statePost).NotTo(BeEmpty(), "Expected '-- state/expect --' section in testdata")
				Expect(resources).NotTo(BeEmpty(), "Expected at least one resource in testdata")
				Expect(stateDelete).NotTo(BeEmpty(), "Expected '-- state/delete --' section in testdata")

				By("preloading gNMI state from testdata")
				serverState := gnmiServer.State()
				serverState.SetBuf([]byte("{}"))
				if len(statePre) != 0 {
					serverState.SetBuf(statePre)
				}

				By("creating test device")
				device := &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-device-",
						Namespace:    testNamespace,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: gnmiServer.GRPCAddr(),
						},
					},
				}
				Expect(k8sClient.Create(ctx, device)).To(Succeed())

				// Set device phase to Running to simulate a ready device for these tests.
				device.Status.Phase = v1alpha1.DevicePhaseRunning
				Expect(k8sClient.Status().Update(ctx, device)).To(Succeed())

				By(fmt.Sprintf("creating %d resource(s) from testdata", len(resources)))
				for _, res := range resources {
					obj := createResourceFromTxtar(ctx, k8sClient, res, device.Name, testNamespace)
					waitForResource(ctx, k8sClient, obj)
				}

				By("verifying gNMI state matches expected JSON")
				Eventually(func(g Gomega) {
					stateJSON := serverState.GetBuf()
					if len(stateJSON) == 0 {
						stateJSON = []byte("{}")
					}
					g.Expect(stateJSON).To(ContainUnorderedJSON(statePost), "gNMI state does not match expected JSON")
				}).Should(Succeed())

				By("deleting all intermeadiate test resources created in test")
				cleanupAllResources(k8sClient, testNamespace)

				By("verifying gNMI state is empty after resource deletion")
				Eventually(func(g Gomega) {
					stateJSON := serverState.GetBuf()
					if len(stateJSON) == 0 {
						stateJSON = []byte("{}")
					}
					g.Expect(stateJSON).To(ContainUnorderedJSON(stateDelete), "gNMI state does not match expected JSON")
				}).Should(Succeed())

				By("deleting the test device")
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
			})
		}
	})
})

// createResourceFromTxtar creates a K8s resource from txtar file data.
// The file name format is "kind/name" (e.g., "prefixset/my-prefixset").
// It substitutes "device" in deviceRef.name with the actual device name.
func createResourceFromTxtar(ctx SpecContext, c client.Client, res txtar.File, deviceName, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(res.Data, obj)).To(Succeed(), "Failed to unmarshal %s", res.Name)

	// Set the namespace
	obj.SetNamespace(namespace)

	// Update deviceRef.name to use the actual device name
	Expect(unstructured.SetNestedField(obj.Object, deviceName, "spec", "deviceRef", "name")).To(Succeed())

	Expect(c.Create(ctx, obj)).To(Succeed(), "Failed to create %s", res.Name)
	return obj
}

// waitForResource waits for a resource to have status conditions set.
// Resources with finalizers have controllers that set Ready/Configured conditions.
// Config-only resources (no finalizer, no controller) will never get conditions,
// so we detect them via a short timeout and skip condition checks.
func waitForResource(ctx SpecContext, c client.Client, obj client.Object) {
	key := client.ObjectKeyFromObject(obj)
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Controllers add finalizers early in reconciliation. Config-only resources
	// have no controller and will never get a finalizer. Poll briefly to detect.
	hasFinalizer := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r := &unstructured.Unstructured{}
		r.SetGroupVersionKind(gvk)
		if err := c.Get(ctx, key, r); err == nil && len(r.GetFinalizers()) > 0 {
			hasFinalizer = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !hasFinalizer {
		return
	}

	// Resource with finalizer — wait for Ready or Configured condition.
	Eventually(func(g Gomega) {
		r := &unstructured.Unstructured{}
		r.SetGroupVersionKind(gvk)
		g.Expect(c.Get(ctx, key, r)).To(Succeed())

		conditions, err := extractConditions(r)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions).NotTo(BeEmpty(), "waiting for first reconcile")

		conditionToCheck := string(v1alpha1.ReadyCondition)
		if apimeta.FindStatusCondition(conditions, string(v1alpha1.ConfiguredCondition)) != nil {
			conditionToCheck = string(v1alpha1.ConfiguredCondition)
		}

		g.Expect(apimeta.IsStatusConditionTrue(conditions, conditionToCheck)).To(BeTrue())
	}).Should(Succeed())
}

// extractConditions extracts status conditions from an unstructured object
// into a typed []metav1.Condition slice for use with apimeta helpers.
func extractConditions(obj *unstructured.Unstructured) ([]metav1.Condition, error) {
	raw, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var conditions []metav1.Condition
	return conditions, json.Unmarshal(data, &conditions)
}

// cleanupAllResources deletes all test resources in the proper order.
//
// This is an envtest workaround. In a real cluster, namespace deletion cascades to
// all resources and the garbage collector handles ordering. But envtest runs without
// kube-controller-manager, so there's no garbage collector and namespace deletion
// just marks the namespace as Terminating without actually deleting anything.
// See: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
//
// The function:
//  1. Deletes resources with finalizers first and waits for their controllers
//     to process the finalizers (cleaning up gNMI state) while Device still exists.
//  2. Deletes config-only resources (no finalizer, no controller)
//     without waiting.
func cleanupAllResources(c client.Client, namespace string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allResources := listAllNetworkOperatorResources(cleanupCtx, c, namespace)

	var withFinalizers, withoutFinalizers []unstructured.Unstructured
	for _, r := range allResources {
		if len(r.GetFinalizers()) > 0 {
			withFinalizers = append(withFinalizers, r)
		} else {
			withoutFinalizers = append(withoutFinalizers, r)
		}
	}

	// Delete resources with finalizers first and wait for controller to process
	for i := range withFinalizers {
		item := &withFinalizers[i]
		Expect(client.IgnoreNotFound(c.Delete(cleanupCtx, item))).To(Succeed())
	}
	for _, item := range withFinalizers {
		Eventually(func(g Gomega) {
			var check unstructured.Unstructured
			check.SetGroupVersionKind(item.GroupVersionKind())
			err := c.Get(cleanupCtx, client.ObjectKeyFromObject(&item), &check)
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(apimeta.IsNoMatchError(err) || err != nil).To(BeTrue())
		}).WithContext(cleanupCtx).Should(Succeed())
	}

	// Delete config resources without finalizers (no wait needed)
	for i := range withoutFinalizers {
		item := &withoutFinalizers[i]
		Expect(client.IgnoreNotFound(c.Delete(cleanupCtx, item))).To(Succeed())
	}
}

// listAllNetworkOperatorResources lists all network-operator CRD instances in a namespace.
func listAllNetworkOperatorResources(ctx context.Context, c client.Client, namespace string) []unstructured.Unstructured {
	var all []unstructured.Unstructured

	for gvk := range scheme.Scheme.AllKnownTypes() {
		if strings.HasSuffix(gvk.Kind, "List") || gvk.Kind == "Device" {
			continue
		}
		if gvk.Group != v1alpha1.GroupVersion.Group && gvk.Group != nxv1alpha1.GroupVersion.Group {
			continue
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		err := c.List(ctx, list, client.InNamespace(namespace))
		if apimeta.IsNoMatchError(err) {
			continue
		}
		Expect(err).NotTo(HaveOccurred())
		all = append(all, list.Items...)
	}
	return all
}
