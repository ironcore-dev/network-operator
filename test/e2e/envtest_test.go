// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build envtest

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/txtar"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nx "github.com/ironcore-dev/network-operator/internal/controller/cisco/nx"
	"github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
	"github.com/ironcore-dev/network-operator/test/e2e/testutil"
)

// reconcileTestNamespacePrefix is used with GenerateName to create unique test namespaces.
// This isolates tests from other resources in the cluster (e.g., from the deployed operator).
const reconcileTestNamespacePrefix = "reconcile-test-"

// testEnv is the envtest test environment.
var testEnv *testutil.EnvtestEnvironment

func init() {
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting network-operator tests in ENVTEST mode\n")
}

// initTestEnv initializes the envtest environment.
func initTestEnv(ctx SpecContext) {
	By("initializing envtest environment")
	testEnv = testutil.NewEnvtestEnvironment()
	Expect(testEnv.Setup(ctx)).To(Succeed())
}

// cleanupTestEnv performs envtest-specific cleanup (none needed).
func cleanupTestEnv(_ SpecContext) {
	// No additional cleanup needed for envtest
}

// ============================================================================
// Provider test helpers
// ============================================================================

// ProviderTestContext holds the context for a provider-specific test run.
type ProviderTestContext struct {
	Provider   testutil.ProviderType
	Manager    ctrl.Manager
	Locker     *resourcelock.ResourceLocker
	Namespace  string
	CancelFunc context.CancelFunc
}

// SetupProviderTest creates a new manager with controllers for the given provider.
// The manager only watches the specified namespace to avoid conflicts with other controllers.
// Call the returned cleanup function in AfterEach/AfterAll.
func SetupProviderTest(providerCfg testutil.ProviderConfig, k8sClient client.Client, restCfg *rest.Config, namespace string) *ProviderTestContext {
	GinkgoHelper()

	providerCtx, providerCancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored in ProviderTestContext

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme: k8sClient.Scheme(),
		Logger: GinkgoLogr,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())

	// Ignore events during tests
	recorder := events.NewFakeRecorder(0)
	go func() {
		for range recorder.Events { //nolint:revive // intentionally drain events
		}
	}()

	locker, err := resourcelock.NewResourceLocker(mgr.GetClient(), namespace, 15*time.Second, 10*time.Second)
	Expect(err).NotTo(HaveOccurred())

	err = mgr.Add(locker)
	Expect(err).NotTo(HaveOccurred())

	providerFunc := providerCfg.NewProvider

	// Register all controllers
	registerControllers(providerCtx, mgr, recorder, providerFunc, locker)

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(providerCtx)
		if providerCtx.Err() == nil {
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}
	}()

	return &ProviderTestContext{
		Provider:   providerCfg.Name,
		Manager:    mgr,
		Locker:     locker,
		Namespace:  namespace,
		CancelFunc: providerCancel,
	}
}

// TeardownProviderTest stops the manager for a provider test.
func TeardownProviderTest(ptc *ProviderTestContext) {
	if ptc != nil && ptc.CancelFunc != nil {
		ptc.CancelFunc()
	}
}

// registerControllers registers all controllers with the manager.
func registerControllers(ctx context.Context, mgr ctrl.Manager, recorder *events.FakeRecorder, providerFunc testutil.ProviderFactory, locker *resourcelock.ResourceLocker) {
	var err error

	err = (&core.PrefixSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.RoutingPolicyReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.InterfaceReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VLANReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VRFReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NTPReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DNSReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.LLDPReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BannerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.OSPFReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.PIMReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NetworkVirtualizationEdgeReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.EVPNInstanceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	// NX-OS specific controllers
	err = (&nx.VPCDomainReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPPeerReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SyslogReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SNMPReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ManagementAccessReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.AccessControlListReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DHCPRelayReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ISISReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())
}

// CreateTestDevice creates a Device pointing to the gNMI server with a generated name.
func CreateTestDevice(ctx context.Context, c client.Client, gnmiAddr, namespace string) (*v1alpha1.Device, error) {
	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-device-",
			Namespace:    namespace,
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: v1alpha1.Endpoint{
				Address: gnmiAddr,
			},
		},
	}
	if err := c.Create(ctx, device); err != nil {
		return nil, err
	}

	// Set the device status to Running so that dependent resources can reconcile
	device.Status.Phase = v1alpha1.DevicePhaseRunning
	if err := c.Status().Update(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

// ============================================================================
// Reconciliation tests
// ============================================================================

var _ = Describe("gNMI requests tests", func() {
	// Resolve provider during tree construction so we can generate individual It nodes
	projectDir, err := testutil.GetProjectDir()
	if err != nil {
		Fail(fmt.Sprintf("Failed to get project directory: %v", err))
	}

	provider := os.Getenv("E2E_PROVIDER")
	providerNames := make([]string, len(testutil.SupportedProviders))
	for i, cfg := range testutil.SupportedProviders {
		providerNames[i] = string(cfg.Name)
	}

	// If provider is invalid, create a failing test with clear message
	if provider == "" {
		It("requires E2E_PROVIDER to be set", func() {
			Fail(fmt.Sprintf("E2E_PROVIDER not set. Please set E2E_PROVIDER to one of: %s", strings.Join(providerNames, ", ")))
		})
		return
	}

	providerIdx := slices.IndexFunc(testutil.SupportedProviders, func(cfg testutil.ProviderConfig) bool {
		return string(cfg.Name) == provider
	})
	if providerIdx < 0 {
		It("requires valid E2E_PROVIDER", func() {
			Fail(fmt.Sprintf("E2E_PROVIDER=%q is not a supported provider. Valid values: %s", provider, strings.Join(providerNames, ", ")))
		})
		return
	}

	providerCfg := testutil.SupportedProviders[providerIdx]
	testdataDir := filepath.Join(projectDir, "test", "e2e", "testdata", string(providerCfg.Name))

	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		It("requires testdata directory", func() {
			Fail(fmt.Sprintf("Testdata directory does not exist for provider %q: %s", provider, testdataDir))
		})
		return
	}

	// Discover test files during tree construction
	testFiles, err := filepath.Glob(filepath.Join(testdataDir, "*.txt"))
	if err != nil || len(testFiles) == 0 {
		It("requires test files", func() {
			if err != nil {
				Fail(fmt.Sprintf("Failed to glob testdata: %v", err))
			}
			Fail(fmt.Sprintf("No test files found in %s", testdataDir))
		})
		return
	}

	Describe(fmt.Sprintf("Provider: %s", providerCfg.Name), Ordered, func() {
		var ptc *ProviderTestContext
		var device *v1alpha1.Device
		var testNamespace string

		BeforeAll(func(ctx SpecContext) {
			By("creating dedicated test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: reconcileTestNamespacePrefix,
				},
			}
			Expect(testEnv.Client().Create(ctx, ns)).To(Succeed())
			testNamespace = ns.Name

			By(fmt.Sprintf("setting up %s provider", providerCfg.Name))
			ptc = SetupProviderTest(providerCfg, testEnv.Client(), testEnv.RESTConfig(), testNamespace)
		})

		AfterAll(func(ctx SpecContext) {
			By(fmt.Sprintf("tearing down %s provider manager", providerCfg.Name))
			TeardownProviderTest(ptc)

			By("deleting test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			_ = testEnv.Client().Delete(ctx, ns)
		})

		AfterEach(func(ctx SpecContext) {
			By("cleaning up resources")
			cleanupAllResources(testEnv.Client(), testNamespace)

			if device == nil {
				return
			}

			By("deleting test device")
			Expect(client.IgnoreNotFound(testEnv.Client().Delete(ctx, device))).To(Succeed())
			Eventually(func(g Gomega) {
				err := testEnv.Client().Get(ctx, client.ObjectKeyFromObject(device), &v1alpha1.Device{})
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
				g.Expect(err).To(HaveOccurred(), "Device should be deleted")
			}).Should(Succeed())
			device = nil

			By("clearing gNMI state for next test")
			Expect(testEnv.ClearGNMIState(ctx)).To(Succeed())
		})

		// Generate individual It nodes for each test file
		for _, testFile := range testFiles {
			testFile := testFile // capture for closure
			testName := filepath.Base(testFile)
			testName = testName[:len(testName)-4] // remove .txt

			It("should reconcile "+testName, func(ctx SpecContext) {
				By("parsing testdata file")
				a, err := txtar.ParseFile(testFile)
				Expect(err).NotTo(HaveOccurred(), "Failed to parse test file: %s", testFile)
				Expect(len(a.Files)).To(BeNumerically(">=", 2), "Expected at least 2 files (resource(s) and state)")

				var state, preload []byte
				var resources []txtar.File
				for _, f := range a.Files {
					switch f.Name {
					case "state/expect":
						state = f.Data
					case "state/preload":
						preload = f.Data
					default:
						resources = append(resources, f)
					}
				}
				Expect(state).NotTo(BeEmpty(), "Expected '-- state/expect --' section in testdata")
				Expect(resources).NotTo(BeEmpty(), "Expected at least one resource in testdata")

				// Preload gNMI state BEFORE creating Device (e.g., bootTime for Device controller)
				if len(preload) > 0 {
					By("preloading gNMI state")
					Expect(testEnv.PreloadGNMIState(ctx, preload)).To(Succeed(), "Failed to preload gNMI state")
				}

				By("creating test device")
				device, err = testutil.CreateTestDevice(ctx, testEnv.Client(), testEnv.GNMIAddress(), testNamespace)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("creating %d resource(s) from testdata", len(resources)))
				for _, res := range resources {
					obj := createResourceFromTxtar(ctx, testEnv.Client(), res, device.Name, testNamespace)
					waitForResource(ctx, testEnv.Client(), obj)
				}

				By("verifying gNMI state matches expected JSON")
				gnmiState, err := testEnv.GetGNMIState(ctx)
				Expect(err).NotTo(HaveOccurred())

				err = testutil.CompareJSON(string(gnmiState), string(state))
				Expect(err).NotTo(HaveOccurred(), "gNMI state does not match expected JSON")
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

	Expect(c.Create(ctx, obj)).To(Succeed(), "Failed to create %s", res.Name)
	return obj
}

// waitForResource waits for a resource to be in Ready=True or Configured=True.
// If Configured condition exists, it checks it, otherwise it falls back to Ready condition.
// Skips config-only --controller-less-- resources that don't have status conditions (e.g., InterfaceConfig).
func waitForResource(ctx SpecContext, c client.Client, obj client.Object) {
	key := client.ObjectKeyFromObject(obj)
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Add as needed.
	switch gvk.Kind {
	case "InterfaceConfig", "LLDPConfig", "BGPConfig", "NVEConfig", "ManagementAccessConfig":
		return
	}

	Eventually(func(g Gomega) {
		r := &unstructured.Unstructured{}
		r.SetGroupVersionKind(gvk)
		g.Expect(c.Get(ctx, key, r)).To(Succeed())

		conditions, err := testutil.ExtractConditions(r)
		g.Expect(err).NotTo(HaveOccurred())

		conditionToCheck := string(v1alpha1.ReadyCondition)
		if apimeta.FindStatusCondition(conditions, string(v1alpha1.ConfiguredCondition)) != nil {
			conditionToCheck = string(v1alpha1.ConfiguredCondition)
		}

		g.Expect(apimeta.IsStatusConditionTrue(conditions, conditionToCheck)).To(BeTrue())
	}).Should(Succeed())
}

// cleanupAllResources deletes all test resources and lets the controller handle finalizer cleanup.
// Uses a background context with timeout to ensure cleanup completes even on interrupt.
// Deletion order: CoreResources first (have finalizers), then ConfigResources, then Device.
func cleanupAllResources(c client.Client, namespace string) {
	// Use background context with timeout - cleanup must complete even on Ctrl+C
	cleanupCtx, cancel := context.WithTimeout(context.Background(), testutil.LongTimeout)
	defer cancel()

	deleteResources := func(gvks []schema.GroupVersionKind) {
		for _, gvk := range gvks {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind + "List",
			})

			if err := c.List(cleanupCtx, list, client.InNamespace(namespace)); err != nil {
				if apimeta.IsNoMatchError(err) {
					continue // CRD not installed, skip
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to list %s", gvk.Kind)
			}

			// Delete all resources - controller will handle finalizer removal
			for _, item := range list.Items {
				Expect(client.IgnoreNotFound(c.Delete(cleanupCtx, &item))).To(Succeed())
			}
		}
	}

	// Delete core resources first (have finalizers that need Device + configs)
	deleteResources(testutil.CoreResources)
	// Then delete config resources
	deleteResources(testutil.ConfigResources)
}
