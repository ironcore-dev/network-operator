// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build envtest

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// BeforeSuite initializes the envtest environment.
// Envtest runs in-process, so no special parallel handling is needed.
var _ = BeforeSuite(func(ctx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	initTestEnv(ctx)
})

// AfterSuite cleans up the envtest environment.
var _ = AfterSuite(func(ctx SpecContext) {
	// Perform cleanup (will run only once even if called from signal handler)
	performCleanup()

	// Run mode-specific cleanup
	cleanupTestEnv(ctx)
})
