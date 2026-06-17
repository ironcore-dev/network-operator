// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build envtest

package e2e

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ironcore-dev/network-operator/test/e2e/testutil"
)

// TestE2E runs the e2e test suite in envtest mode.
func TestEnvtest(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting network-operator tests in ENVTEST mode\n")
	RunSpecs(t, "e2e suite (envtest)")
}

// BeforeSuite initializes the envtest environment.
// Envtest runs in-process, so no special parallel handling is needed.
var _ = BeforeSuite(func(ctx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	SetDefaultEventuallyTimeout(testutil.DefaultTimeout)
	SetDefaultEventuallyPollingInterval(time.Second)

	initTestEnv(ctx)
})

// AfterSuite cleans up the envtest environment.
var _ = AfterSuite(func(ctx SpecContext) {
	fmt.Fprintf(GinkgoWriter, "Tearing down test environment...\n")
	cleanupTestEnv(ctx)
})
