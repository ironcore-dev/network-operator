// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	poolcontroller "github.com/ironcore-dev/network-operator/internal/controller/pool"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx        context.Context
	cancel     context.CancelFunc
	testEnv    *envtest.Environment
	k8sClient  client.Client
	k8sManager ctrl.Manager
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	SetDefaultEventuallyTimeout(time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	Expect(evpnv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(corev1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(poolv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Logger:                 GinkgoLogr,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	recorder := events.NewFakeRecorder(100)
	go func() {
		defer GinkgoRecover()
		for event := range recorder.Events {
			GinkgoLogr.Info("Event", "event", event)
		}
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	err = (&FabricReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: NewProvider,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&poolcontroller.IPAddressPoolReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&poolcontroller.IPAddressReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&poolcontroller.ClaimReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err := k8sManager.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()

	Eventually(func() error {
		var namespace corev1.Namespace
		return k8sClient.Get(context.Background(), client.ObjectKey{Name: metav1.NamespaceDefault}, &namespace)
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

var _ provider.InterfaceProvider = (*Provider)(nil)

type Provider struct{}

func NewProvider() provider.Provider { return &Provider{} }

func (p *Provider) Connect(context.Context, *deviceutil.Connection) error    { return nil }
func (p *Provider) Disconnect(context.Context, *deviceutil.Connection) error { return nil }
func (p *Provider) EnsureInterface(context.Context, *provider.EnsureInterfaceRequest) error {
	return nil
}

func (p *Provider) DeleteInterface(context.Context, *provider.InterfaceRequest) error { return nil }

func (p *Provider) GetInterfaceStatus(context.Context, *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	return provider.InterfaceStatus{}, nil
}

func (p *Provider) InterfaceNameEqual(_ context.Context, a, b string) (bool, error) {
	return a == b, nil
}

func (p *Provider) LoopbackInterfaceName(id int) (string, error) {
	return fmt.Sprintf("lo%d", id), nil
}
