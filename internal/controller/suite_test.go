// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx          context.Context
	cancel       context.CancelFunc
	testEnv      *envtest.Environment
	k8sClient    client.Client
	k8sManager   ctrl.Manager
	testProvider = NewProvider()
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	err := corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if dir := detectTestBinaryDir(); dir != "" {
		testEnv.BinaryAssetsDirectory = dir
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Logger: GinkgoLogr,
	})
	Expect(err).ToNot(HaveOccurred())

	recorder := record.NewFakeRecorder(0)
	go func() {
		for event := range recorder.Events {
			GinkgoLogr.Info("Event", "event", event)
		}
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	prov := func() provider.Provider { return testProvider }

	err = (&DeviceReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&InterfaceReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&BannerReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&UserReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&DNSReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
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

// detectTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func detectTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
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

var (
	_ provider.Provider          = (*Provider)(nil)
	_ provider.InterfaceProvider = (*Provider)(nil)
	_ provider.BannerProvider    = (*Provider)(nil)
	_ provider.UserProvider      = (*Provider)(nil)
	_ provider.DNSProvider       = (*Provider)(nil)
)

// Provider is a simple in-memory provider for testing purposes only.
type Provider struct {
	sync.Mutex

	Items  map[string]client.Object
	User   map[string]struct{}
	Banner *string
	DNS    *v1alpha1.DNS
	NTP    *v1alpha1.NTP
}

func NewProvider() *Provider {
	return &Provider{
		Items: make(map[string]client.Object),
		User:  make(map[string]struct{}),
	}
}

func (p *Provider) Connect(context.Context, *deviceutil.Connection) error    { return nil }
func (p *Provider) Disconnect(context.Context, *deviceutil.Connection) error { return nil }

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) (provider.Result, error) {
	p.Lock()
	defer p.Unlock()
	p.Items[req.Interface.Name] = req.Interface
	return provider.Result{}, nil
}

func (p *Provider) DeleteInterface(_ context.Context, req *provider.InterfaceRequest) error {
	p.Lock()
	defer p.Unlock()
	delete(p.Items, req.Interface.Name)
	return nil
}

func (p *Provider) EnsureBanner(_ context.Context, req *provider.BannerRequest) (provider.Result, error) {
	p.Lock()
	defer p.Unlock()
	p.Banner = &req.Message
	return provider.Result{}, nil
}

func (p *Provider) DeleteBanner(context.Context) error {
	p.Lock()
	defer p.Unlock()
	p.Banner = nil
	return nil
}

func (p *Provider) EnsureUser(_ context.Context, req *provider.EnsureUserRequest) (provider.Result, error) {
	p.Lock()
	defer p.Unlock()
	p.User[req.Username] = struct{}{}
	return provider.Result{}, nil
}

func (p *Provider) DeleteUser(_ context.Context, req *provider.DeleteUserRequest) error {
	p.Lock()
	defer p.Unlock()
	delete(p.User, req.Username)
	return nil
}

func (p *Provider) EnsureDNS(_ context.Context, req *provider.EnsureDNSRequest) (provider.Result, error) {
	p.Lock()
	defer p.Unlock()
	p.DNS = req.DNS
	return provider.Result{}, nil
}

func (p *Provider) DeleteDNS(_ context.Context) error {
	p.Lock()
	defer p.Unlock()
	p.DNS = nil
	return nil
}
