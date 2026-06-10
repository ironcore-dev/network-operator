// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

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

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/iosxr"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"

	gnmitestserver "github.com/ironcore-dev/gnmi-test-server/testserver"
)

// ProviderType represents the network device provider to test against.
type ProviderType string

// ProviderFactory creates a new provider instance.
type ProviderFactory = func() provider.Provider

const (
	ProviderNXOS  ProviderType = "nxos"
	ProviderIOSXR ProviderType = "iosxr"
)

// ProviderConfig holds the configuration for a provider test.
type ProviderConfig struct {
	Name        ProviderType
	NewProvider ProviderFactory
}

// SupportedProviders lists all providers to test.
var SupportedProviders = []ProviderConfig{
	{Name: ProviderNXOS, NewProvider: func() provider.Provider { return nxos.NewProvider() }},
	{Name: ProviderIOSXR, NewProvider: func() provider.Provider { return iosxr.NewProvider() }},
}

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	k8sClient client.Client
	restCfg   *rest.Config

	// gNMI test server
	gnmiServer *gnmitestserver.Server
	gnmiAddr   string
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	By("starting in-process gNMI test server")
	var err error
	gnmiServer, gnmiAddr, _, err = gnmitestserver.NewTestServer(ctx, gnmitestserver.WithNXOSBehavior())
	Expect(err).NotTo(HaveOccurred())
	GinkgoLogr.Info("gNMI server started", "grpc", gnmiAddr)

	By("bootstrapping test environment")
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = nxv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if dir := detectTestBinaryDir(); dir != "" {
		testEnv.BinaryAssetsDirectory = dir
	}

	restCfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	k8sClient, err = client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Eventually(func() error {
		var namespace corev1.Namespace
		return k8sClient.Get(ctx, client.ObjectKey{Name: metav1.NamespaceDefault}, &namespace)
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	var errs []error
	if gnmiServer != nil {
		if err := gnmiServer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if testEnv != nil {
		if err := testEnv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	Expect(errors.Join(errs...)).NotTo(HaveOccurred(), "errors during teardown")
})

// ProviderTestContext holds the context for a provider-specific test run.
type ProviderTestContext struct {
	Provider   ProviderType
	Manager    ctrl.Manager
	Locker     *resourcelock.ResourceLocker
	CancelFunc context.CancelFunc
}

// SetupProviderTest creates a new manager with controllers for the given provider.
// Call the returned cleanup function in AfterEach/AfterAll.
func SetupProviderTest(providerCfg ProviderConfig) *ProviderTestContext {
	GinkgoHelper()

	providerCtx, providerCancel := context.WithCancel(ctx) //nolint:gosec // cancel stored in ProviderTestContext

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Logger: GinkgoLogr,
	})
	Expect(err).ToNot(HaveOccurred())

	// Ignore events during tests
	recorder := events.NewFakeRecorder(0)
	go func() {
		for range recorder.Events { //nolint:revive // intentionally drain events
		}
	}()

	locker, err := resourcelock.NewResourceLocker(mgr.GetClient(), metav1.NamespaceDefault, 15*time.Second, 10*time.Second)
	Expect(err).NotTo(HaveOccurred())

	err = mgr.Add(locker)
	Expect(err).NotTo(HaveOccurred())

	providerFunc := providerCfg.NewProvider

	// Register the controllers
	err = (&core.PrefixSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.RoutingPolicyReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.InterfaceReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VLANReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VRFReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NTPReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DNSReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.LLDPReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BannerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.OSPFReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.PIMReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
		Provider: providerFunc,
		Locker:   locker,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NetworkVirtualizationEdgeReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        recorder,
		Provider:        providerFunc,
		Locker:          locker,
		RequeueInterval: time.Minute,
	}).SetupWithManager(providerCtx, mgr)
	Expect(err).NotTo(HaveOccurred())

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
		CancelFunc: providerCancel,
	}
}

// TeardownProviderTest stops the manager for a provider test.
func TeardownProviderTest(ptc *ProviderTestContext) {
	if ptc != nil && ptc.CancelFunc != nil {
		ptc.CancelFunc()
	}
}

// detectTestBinaryDir locates the first directory in the k8s binary path.
func detectTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	idx := slices.IndexFunc(entries, func(e os.DirEntry) bool {
		return e.IsDir()
	})
	if idx >= 0 {
		return filepath.Join(basePath, entries[idx].Name())
	}
	return ""
}

// GNMIAddr returns the gNMI server address for tests to use
func GNMIAddr() string {
	return gnmiAddr
}

// GetGNMIState returns the current accumulated gNMI state as JSON
func GetGNMIState() ([]byte, error) {
	return gnmiServer.GetState()
}

// ClearGNMIState clears the accumulated gNMI state
func ClearGNMIState() {
	gnmiServer.ClearState()
}

// NewGNMIConnection creates a gRPC connection to the in-process gNMI server
func NewGNMIConnection() (*grpc.ClientConn, error) {
	return grpc.NewClient(
		gnmiAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		})),
	)
}

// CreateTestDevice creates a Device pointing to the in-process gNMI server.
// If name is empty, a unique name is generated using GenerateName.
func CreateTestDevice(ctx context.Context, c client.Client, name string) (*v1alpha1.Device, error) {
	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: v1alpha1.Endpoint{
				Address: gnmiAddr,
			},
		},
	}
	if name == "" {
		device.GenerateName = "test-device-"
	} else {
		device.Name = name
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

// TestdataPath returns the path to testdata for the given provider.
// Example: testdata/nxos/interfaces.txt
func TestdataPath(provider ProviderType, filename string) string {
	return filepath.Join("testdata", string(provider), filename)
}
