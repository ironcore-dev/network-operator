// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmi_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	coordinationv1 "k8s.io/api/coordination/v1"
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

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
	"github.com/ironcore-dev/network-operator/test/gnmi/testserver"
)

// Integration test variables - separate from existing gnmi_suite_test.go
var (
	integrationCtx     context.Context
	integrationCancel  context.CancelFunc
	integrationTestEnv *envtest.Environment
	integrationClient  client.Client
	integrationManager ctrl.Manager
	integrationServer  *testserver.Server
	integrationGRPCAddr string
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gNMI Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	integrationCtx, integrationCancel = context.WithCancel(ctrl.SetupSignalHandler())

	By("starting gNMI test server with NX-OS behavior")
	var err error
	integrationServer, integrationGRPCAddr, _, err = testserver.NewTestServer(integrationCtx, testserver.WithNXOSBehavior())
	Expect(err).NotTo(HaveOccurred(), "Failed to start gNMI test server")
	GinkgoWriter.Printf("gNMI server started at %s\n", integrationGRPCAddr)

	By("registering schemes")
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = nxv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	integrationTestEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Detect test binary directory for IDEs
	if dir := detectIntegrationTestBinaryDir(); dir != "" {
		integrationTestEnv.BinaryAssetsDirectory = dir
	}

	cfg, err := integrationTestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	By("creating controller manager")
	integrationManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Logger: GinkgoLogr,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server to avoid port conflicts
		},
	})
	Expect(err).ToNot(HaveOccurred())

	By("creating event recorder")
	recorder := events.NewFakeRecorder(100)
	go func() {
		for event := range recorder.Events {
			GinkgoLogr.V(1).Info("Event", "event", event)
		}
	}()

	By("creating k8s client")
	integrationClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(integrationClient).NotTo(BeNil())

	By("setting up resource locker")
	testLocker, err := resourcelock.NewResourceLocker(integrationManager.GetClient(), metav1.NamespaceDefault, 15*time.Second, 10*time.Second)
	Expect(err).NotTo(HaveOccurred())
	err = integrationManager.Add(testLocker)
	Expect(err).NotTo(HaveOccurred())

	// Set up cache informer for Lease resources used by ResourceLocker
	_, err = integrationManager.GetCache().GetInformer(integrationCtx, &coordinationv1.Lease{})
	Expect(err).NotTo(HaveOccurred())

	By("registering reconcilers with nxos provider")
	// Provider factory returns a fresh nxos provider each call
	// The provider will connect to gNMI server when reconciling Device
	prov := func() provider.Provider { return nxos.NewProvider() }

	err = (&core.DeviceReconciler{
		Client:            integrationManager.GetClient(),
		Scheme:            integrationManager.GetScheme(),
		Recorder:          recorder,
		Provider:          prov,
		HeartbeatInterval: time.Second,
	}).SetupWithManager(integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.InterfaceReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BannerReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DNSReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NTPReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.AccessControlListReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SNMPReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.SyslogReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ManagementAccessReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.ISISReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VRFReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.PIMReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.BGPPeerReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.OSPFReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.VLANReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.EVPNInstanceReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.NetworkVirtualizationEdgeReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.PrefixSetReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.RoutingPolicyReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.LLDPReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.DHCPRelayReconciler{
		Client:          integrationManager.GetClient(),
		Scheme:          integrationManager.GetScheme(),
		Recorder:        recorder,
		Provider:        prov,
		Locker:          testLocker,
		RequeueInterval: time.Second,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&core.EthernetSegmentReconciler{
		Client:   integrationManager.GetClient(),
		Scheme:   integrationManager.GetScheme(),
		Recorder: recorder,
		Provider: prov,
		Locker:   testLocker,
	}).SetupWithManager(integrationCtx, integrationManager)
	Expect(err).NotTo(HaveOccurred())

	By("starting controller manager")
	go func() {
		defer GinkgoRecover()
		err = integrationManager.Start(integrationCtx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	By("waiting for default namespace")
	Eventually(func() error {
		var namespace corev1.Namespace
		return integrationClient.Get(integrationCtx, client.ObjectKey{Name: metav1.NamespaceDefault}, &namespace)
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down test environment")
	integrationCancel()

	if integrationServer != nil {
		err := integrationServer.Close()
		Expect(err).NotTo(HaveOccurred())
	}

	if integrationTestEnv != nil {
		err := integrationTestEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

// detectIntegrationTestBinaryDir locates the envtest binary directory.
// The structure is: bin/k8s/k8s/<version>-<os>-<arch>/
func detectIntegrationTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	// Find a version directory (e.g., "1.35.0-darwin-arm64")
	idx := slices.IndexFunc(entries, func(e os.DirEntry) bool {
		if !e.IsDir() {
			return false
		}
		// Check if this directory contains etcd
		etcdPath := filepath.Join(basePath, e.Name(), "etcd")
		_, err := os.Stat(etcdPath)
		return err == nil
	})
	if idx >= 0 {
		return filepath.Join(basePath, entries[idx].Name())
	}
	return ""
}
