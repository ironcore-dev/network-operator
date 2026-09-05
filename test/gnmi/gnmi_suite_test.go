// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmitest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nxcontroller "github.com/ironcore-dev/network-operator/internal/controller/cisco/nx"
	"github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
	testserver "github.com/ironcore-dev/network-operator/test/gnmi/server"

	// Import all supported provider implementations.
	_ "github.com/ironcore-dev/network-operator/internal/provider/cisco/iosxr"
	_ "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	_ "github.com/ironcore-dev/network-operator/internal/provider/openconfig"
)

const (
	ProviderEnvVar = "PROVIDER"
)

// Test environment — initialized in BeforeSuite
var (
	testEnv    *envtest.Environment
	restConfig *rest.Config
	k8sClient  client.Client
	gnmiServer *testserver.Server
)

// providerFunc is the provider function resolved during Ginkgo tree construction.
// NOTE: This is assigned in the Describe block (gnmi_test.go) which runs BEFORE BeforeSuite.
var providerFunc provider.ProviderFunc

// suiteCancel stops long-running components (gNMI server, controller manager) in AfterSuite.
var suiteCancel context.CancelFunc

// TestGNMI runs the gNMI integration test suite.
func TestGNMI(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting network-operator gNMI integration tests\n")
	RunSpecs(t, "gNMI Integration Suite")
}

// BeforeSuite initializes the test environment.
// It starts the gNMI test server, sets up the Kubernetes client, and starts the controller manager.
var _ = BeforeSuite(func(ctx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	format.MaxLength = 0
	SetDefaultEventuallyTimeout(60 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	By("resolving provider")
	// PROVIDER env var already validated during tree construction (Describe block runs first)
	var err error
	providerFunc, err = provider.Get(os.Getenv(ProviderEnvVar))
	Expect(err).NotTo(HaveOccurred())

	By("initializing envtest environment")

	// Create a context for long-running servers that outlives BeforeSuite.
	// Ginkgo's ctx is block-scoped and cancelled when BeforeSuite returns,
	// but servers must run until AfterSuite calls suiteCancel().
	var suiteCtx context.Context
	suiteCtx, suiteCancel = context.WithCancel(context.Background())

	gnmiServer, err = testserver.NewTestServer(suiteCtx)
	Expect(err).NotTo(HaveOccurred())

	// Register schemas, add as needed.
	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(nxv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	// Start envtest (uses KUBEBUILDER_ASSETS env var or default location)
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() error {
		var ns corev1.Namespace
		return k8sClient.Get(ctx, client.ObjectKey{Name: metav1.NamespaceDefault}, &ns)
	}).Should(Succeed())

	By("starting controller manager")
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:  k8sClient.Scheme(),
		Logger:  GinkgoLogr,
		Metrics: metricsserver.Options{BindAddress: "0"}, // Disable metrics server
	})
	Expect(err).ToNot(HaveOccurred())

	recorder := events.NewFakeRecorder(0)

	locker, err := resourcelock.NewResourceLocker(mgr.GetClient(), metav1.NamespaceDefault, 15*time.Second, 10*time.Second)
	Expect(err).NotTo(HaveOccurred())

	err = mgr.Add(locker)
	Expect(err).NotTo(HaveOccurred())

	registerControllers(ctx, mgr, recorder, providerFunc, locker)

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(suiteCtx)
		if suiteCtx.Err() == nil {
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}
	}()

	By("waiting for manager cache to sync")
	Expect(mgr.GetCache().WaitForCacheSync(suiteCtx)).To(BeTrue())
})

// AfterSuite cleans up the test environment.
var _ = AfterSuite(func(ctx SpecContext) {
	fmt.Fprintf(GinkgoWriter, "Tearing down test environment...\n")
	if suiteCancel != nil {
		suiteCancel()
	}

	if gnmiServer != nil {
		gnmiServer.Close()
	}

	if testEnv != nil {
		_ = testEnv.Stop() //nolint:errcheck // best-effort cleanup in AfterSuite
	}
})

// registerControllers registers all controllers with the manager.
// Add more controllers here as needed for testing.
func registerControllers(ctx context.Context, mgr ctrl.Manager, recorder *events.FakeRecorder, providerFn provider.ProviderFunc, locker *resourcelock.ResourceLocker) {
	var err error

	err = (&core.PrefixSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.RoutingPolicyReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.InterfaceReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VLANReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VRFReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NTPReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DNSReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.LLDPReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.AAAReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BannerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.UserReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.OSPFReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.PIMReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NetworkVirtualizationEdgeReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.EVPNInstanceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPPeerReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SyslogReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SNMPReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ManagementAccessReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.AccessControlListReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DHCPRelayReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFn,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ISISReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&nxcontroller.SystemReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&nxcontroller.VPCDomainReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&nxcontroller.BorderGatewayReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFn,
		Locker:   locker,
	}).SetupWithManager(ctx, mgr)
	Expect(err).NotTo(HaveOccurred())
}
