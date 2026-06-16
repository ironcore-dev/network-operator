// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package testutil provides test infrastructure for e2e tests.
//
// It supports two test modes selected by build tags:
//
//   - Envtest (build tag: envtest): Uses controller-runtime's envtest.Environment
//     with an in-process gNMI test server. Fast (~10s) but doesn't test deployment.
//
//   - Cluster (default): Uses a real Kubernetes cluster (typically Kind) with a
//     deployed operator and gnmi-test-server pod. Slower (~2-5min) but tests full stack.
//
// The concrete types ClusterEnvironment and EnvtestEnvironment provide the same
// methods, allowing test logic to work with either mode via build tag selection.
package testutil
