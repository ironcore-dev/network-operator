// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ironcore-dev/network-operator/test/e2e/testutil"
)

var (
	// Optional Environment Variables:
	// - PROMETHEUS_INSTALL_SKIP=true: Skips Prometheus Operator installation during test setup.
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// These variables are useful if Prometheus or CertManager is already installed, avoiding re-installation and conflicts.
	skipPrometheusInstall  = os.Getenv("PROMETHEUS_INSTALL_SKIP") == "true"
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	// isPrometheusOperatorAlreadyInstalled will be set true when prometheus CRDs be found on the cluster
	isPrometheusOperatorAlreadyInstalled = false
	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false
)

// image is the name of the image which will be build and loaded
// with the code source changes to be tested.
const image = "ghcr.io/ironcore-dev/network-operator:latest"

// serverImage is the name of the image which will be built and loaded
// with the gNMI test server.
const serverImage = "ghcr.io/ironcore-dev/gnmi-test-server:latest"

var (
	cleanupOnce sync.Once
	cleanupCtx  context.Context
	cleanupDone = make(chan struct{})
)

// TestE2E runs the end-to-end (e2e) test suite for the project.
//
// Build with -tags=envtest to run in envtest mode (fast, in-process controllers).
// Build without tags to run in cluster mode (requires Kind cluster).
func TestE2E(t *testing.T) {
	// Setup signal handler to ensure cleanup on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	cleanupCtx = ctx

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\n\nReceived interrupt signal, cleaning up test environment...\n")
		performCleanup()
		cancel()
		close(cleanupDone)
		os.Exit(1)
	}()

	RegisterFailHandler(Fail)
	RunSpecs(t, "e2e suite")
}

// performCleanup ensures testEnv.Teardown is called exactly once
func performCleanup() {
	cleanupOnce.Do(func() {
		if testEnv != nil {
			fmt.Fprintf(os.Stderr, "Tearing down test environment...\n")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := testEnv.Teardown(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to teardown test environment: %v\n", err)
			}
		}
	})
}

// BeforeSuite and AfterSuite are defined in mode-specific files:
// - envtest_test.go (build tag: envtest) - simple BeforeSuite/AfterSuite
// - cluster_suite_test.go (build tag: !envtest) - SynchronizedBeforeSuite/AfterSuite for parallel execution

// setupClusterDependencies installs Prometheus and CertManager if needed.
// Called by cluster_suite_test.go.
func setupClusterDependencies(ctx SpecContext) {
	if !skipPrometheusInstall {
		By("checking if prometheus is installed already")
		isPrometheusOperatorAlreadyInstalled = testutil.IsPrometheusCRDsInstalled(ctx, GinkgoWriter)
		if !isPrometheusOperatorAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing Prometheus Operator...\n")
			Expect(testutil.InstallPrometheusOperator(ctx, GinkgoWriter)).To(Succeed(), "Failed to install Prometheus Operator")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: Prometheus Operator is already installed. Skipping installation...\n")
		}
	}
	if !skipCertManagerInstall {
		By("checking if cert manager is installed already")
		isCertManagerAlreadyInstalled = testutil.IsCertManagerCRDsInstalled(ctx, GinkgoWriter)
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")
			Expect(testutil.InstallCertManager(ctx, GinkgoWriter)).To(Succeed(), "Failed to install CertManager")
			// Fresh install - need to wait for webhook to be ready (can take up to 90s)
			By("waiting for cert-manager webhook to be ready (fresh install)")
			Expect(testutil.WaitForCertManagerWebhook(ctx, GinkgoWriter)).To(Succeed(), "Cert-manager webhook not ready")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
			// Already installed - webhook should be ready, but verify quickly
		}
	}
}

// cleanupClusterDependencies uninstalls Prometheus and CertManager if we installed them.
// Called by cluster_suite_test.go.
func cleanupClusterDependencies(ctx SpecContext) {
	if !skipPrometheusInstall && !isPrometheusOperatorAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling Prometheus Operator...\n")
		testutil.UninstallPrometheusOperator(ctx, GinkgoWriter)
	}
	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		testutil.UninstallCertManager(ctx, GinkgoWriter)
	}
}

// buildAndLoadImages builds and loads Docker images to Kind.
// Called by cluster_suite_test.go.
func buildAndLoadImages(ctx SpecContext) {
	By("building the manager(Operator) image")
	cmd := exec.CommandContext(ctx, "make", "docker-build", "IMG="+image)
	_, err := testutil.Run(cmd, GinkgoWriter)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

	By("loading the manager(Operator) image on Kind")
	err = testutil.LoadImageToKindClusterWithName(ctx, image, GinkgoWriter)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

	By("building the gnmi-test-server image")
	cmd = exec.CommandContext(ctx, "make", "docker-build-test-gnmi-server", "TEST_SERVER_IMG="+serverImage)
	_, err = testutil.Run(cmd, GinkgoWriter)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the gnmi-test-server image")

	By("loading the gnmi-test-server image on Kind")
	err = testutil.LoadImageToKindClusterWithName(ctx, serverImage, GinkgoWriter)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the gnmi-test-server image into Kind")
}
