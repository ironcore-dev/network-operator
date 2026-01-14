// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
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
	testProvider = NewMockProvider()
	testLocker   *resourcelock.ResourceLocker
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cisco NX Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	SetDefaultEventuallyTimeout(time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	err := corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = nxv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
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

	testLocker, err = resourcelock.NewResourceLocker(k8sManager.GetClient(), metav1.NamespaceDefault, 15*time.Second, 10*time.Second)
	Expect(err).NotTo(HaveOccurred())

	err = k8sManager.Add(testLocker)
	Expect(err).NotTo(HaveOccurred())

	// Set up cache informer for Lease resources used by ResourceLocker
	_, err = k8sManager.GetCache().GetInformer(ctx, &coordinationv1.Lease{})
	Expect(err).NotTo(HaveOccurred())

	prov := func() provider.Provider { return testProvider }

	err = (&SystemReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&VPCDomainReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   scheme.Scheme,
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&BorderGatewayReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(ctx, k8sManager)
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
	basePath := filepath.Join("..", "..", "..", "..", "bin", "k8s")
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

type MockProvider struct {
	sync.Mutex

	BorderGateway *nxv1alpha1.BorderGateway
	NVE           *v1alpha1.NetworkVirtualizationEdge
	Settings      *nxv1alpha1.System
	VPCDomain     *nxv1alpha1.VPCDomain
}

var _ Provider = (*MockProvider)(nil)

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Connect(context.Context, *deviceutil.Connection) error    { return nil }
func (p *MockProvider) Disconnect(context.Context, *deviceutil.Connection) error { return nil }

func (p *MockProvider) EnsureSystemSettings(ctx context.Context, s *nxv1alpha1.System) error {
	p.Lock()
	defer p.Unlock()
	p.Settings = s
	return nil
}

func (p *MockProvider) ResetSystemSettings(ctx context.Context) error {
	p.Lock()
	defer p.Unlock()
	p.Settings = nil
	return nil
}

func (p *MockProvider) EnsureVPCDomain(_ context.Context, vpc *nxv1alpha1.VPCDomain, _ *v1alpha1.VRF, _ *v1alpha1.Interface) error {
	p.Lock()
	defer p.Unlock()
	p.VPCDomain = vpc
	return nil
}

func (p *MockProvider) DeleteVPCDomain(_ context.Context) error {
	p.Lock()
	defer p.Unlock()
	p.VPCDomain = nil
	return nil
}

func (p *MockProvider) GetStatusVPCDomain(_ context.Context) (nxos.VPCDomainStatus, error) {
	return nxos.VPCDomainStatus{
		KeepAliveStatus:    true,
		KeepAliveStatusMsg: []string{"operational"},
		PeerStatus:         true,
		PeerStatusMsg:      []string{"success"},
		PeerUptime:         3600 * time.Second,
		Role:               nxv1alpha1.VPCDomainRolePrimary,
	}, nil
}

func (p *MockProvider) EnsureBorderGatewaySettings(ctx context.Context, req *nxos.BorderGatewaySettingsRequest) error {
	p.Lock()
	defer p.Unlock()
	p.BorderGateway = req.BorderGateway
	return nil
}

func (p *MockProvider) ResetBorderGatewaySettings(ctx context.Context) error {
	p.Lock()
	defer p.Unlock()
	p.BorderGateway = nil
	return nil
}

func (p *MockProvider) EnsureNVE(_ context.Context, req *provider.NVERequest) error {
	p.Lock()
	defer p.Unlock()
	p.NVE = req.NVE
	return nil
}

func (p *MockProvider) DeleteNVE(_ context.Context, req *provider.NVERequest) error {
	p.Lock()
	defer p.Unlock()
	p.NVE = nil
	return nil
}

func (p *MockProvider) GetNVEStatus(_ context.Context, req *provider.NVERequest) (provider.NVEStatus, error) {
	status := provider.NVEStatus{
		OperStatus: true,
	}
	if p.NVE != nil {
		if p.NVE.Spec.SourceInterfaceRef.Name != "" {
			status.SourceInterfaceName = p.NVE.Spec.SourceInterfaceRef.Name
		}
		if p.NVE.Spec.AnycastSourceInterfaceRef != nil {
			status.AnycastSourceInterfaceName = p.NVE.Spec.AnycastSourceInterfaceRef.Name
		}
	}
	return status, nil
}
