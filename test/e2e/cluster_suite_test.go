// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build !envtest

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ironcore-dev/network-operator/test/e2e/testutil"
)

// SynchronizedBeforeSuite enables parallel test execution:
// - Process 1: Builds images, installs Prometheus/CertManager, deploys manager (runs first, alone)
// - All processes: Create ClusterEnvironment connection (runs after process 1 completes)
var _ = SynchronizedBeforeSuite(
	// First function: runs ONLY on process 1, before other processes start
	func(ctx SpecContext) []byte {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		SetDefaultEventuallyTimeout(30 * time.Second)
		SetDefaultEventuallyPollingInterval(time.Second)

		By("Ensure that Prometheus is enabled")
		cwd, err := testutil.GetProjectDir()
		Expect(err).NotTo(HaveOccurred(), "Failed to get project directory")

		err = testutil.UncommentCode(cwd+"/config/default/kustomization.yaml", "#- ../prometheus", "#")
		Expect(err).NotTo(HaveOccurred(), "Failed to enable Prometheus")

		// Build and load images to Kind (only process 1)
		buildAndLoadImages(ctx)

		// Setup Prometheus and CertManager (only process 1)
		setupClusterDependencies(ctx)

		// Deploy controller-manager (includes CRDs via make deploy)
		By("deploying controller-manager")
		tmpEnv := testutil.NewClusterEnvironment()
		Expect(tmpEnv.Setup(ctx)).To(Succeed())
		Expect(tmpEnv.DeployManager(ctx)).To(Succeed())

		return nil // No data to pass to other processes
	},
	// Second function: runs on ALL processes after the first function completes
	func(ctx SpecContext, _ []byte) {
		SetDefaultEventuallyTimeout(30 * time.Second)
		SetDefaultEventuallyPollingInterval(time.Second)

		// All processes create their own ClusterEnvironment connection
		By("initializing cluster environment")
		testEnv = testutil.NewClusterEnvironment()
		Expect(testEnv.Setup(ctx)).To(Succeed())
	},
)

// SynchronizedAfterSuite enables parallel test cleanup:
// - All processes: Local cleanup (runs on all processes)
// - Process 1: Uninstall shared dependencies (runs last, alone)
var _ = SynchronizedAfterSuite(
	// First function: runs on ALL processes
	func(ctx SpecContext) {
		// Perform local cleanup (will run only once even if called from signal handler)
		performCleanup()

		// Wait for all test namespaces to be fully deleted before returning.
		// This ensures DeferCleanup hooks have finished deleting resources and their
		// finalizers have been processed by the controller. Without this, the second
		// function (UndeployManager) may delete the CRDs while resources still exist,
		// causing finalizers to be stuck forever.
		if testEnv != nil {
			_ = testEnv.WaitForTestNamespacesGone(ctx)
		}
	},
	// Second function: runs ONLY on process 1, after all other processes complete
	func(ctx SpecContext) {
		// Undeploy the controller-manager
		tmpEnv := testutil.NewClusterEnvironment()
		_ = tmpEnv.Setup(ctx)
		_ = tmpEnv.UndeployManager(ctx)

		// Uninstall Prometheus and CertManager
		cleanupClusterDependencies(ctx)
	},
)
