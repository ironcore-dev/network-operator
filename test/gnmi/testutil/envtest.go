// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/test/gnmi/testserver"
)

// EnvtestEnvironment implements TestEnvironment using envtest and an in-process gNMI server.
type EnvtestEnvironment struct {
	testEnv    *envtest.Environment
	restConfig *rest.Config
	k8sClient  client.Client
	gnmiServer *testserver.Server
	gnmiAddr   string
	cancel     context.CancelFunc
}

// NewEnvtestEnvironment creates a new envtest-based test environment.
func NewEnvtestEnvironment() *EnvtestEnvironment {
	return &EnvtestEnvironment{}
}

// Setup initializes envtest and starts the in-process gNMI server.
func (e *EnvtestEnvironment) Setup(ctx context.Context) error {
	ctx, e.cancel = context.WithCancel(ctx)

	// Start in-process gNMI test server with NX-OS behavior
	var err error
	e.gnmiServer, e.gnmiAddr, _, err = testserver.NewTestServer(ctx, testserver.WithNXOSBehavior())
	if err != nil {
		return err
	}

	// Register schemes
	if err := corev1.AddToScheme(scheme.Scheme); err != nil {
		return err
	}
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return err
	}
	if err := nxv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	// Start envtest - CRD path is relative to test/gnmi/
	e.testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Detect test binary directory for IDEs
	if dir := detectTestBinaryDir(); dir != "" {
		e.testEnv.BinaryAssetsDirectory = dir
	}

	e.restConfig, err = e.testEnv.Start()
	if err != nil {
		return err
	}

	e.k8sClient, err = client.New(e.restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	// Wait for default namespace to be ready
	for {
		var ns corev1.Namespace
		if err := e.k8sClient.Get(ctx, client.ObjectKey{Name: metav1.NamespaceDefault}, &ns); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// Teardown stops envtest and the gNMI server.
func (e *EnvtestEnvironment) Teardown(_ context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}

	if e.gnmiServer != nil {
		if err := e.gnmiServer.Close(); err != nil {
			return err
		}
	}

	if e.testEnv != nil {
		if err := e.testEnv.Stop(); err != nil {
			return err
		}
	}

	return nil
}

// Client returns the Kubernetes client.
func (e *EnvtestEnvironment) Client() client.Client {
	return e.k8sClient
}

// RESTConfig returns the REST config.
func (e *EnvtestEnvironment) RESTConfig() *rest.Config {
	return e.restConfig
}

// GNMIAddress returns the in-process gNMI server address.
func (e *EnvtestEnvironment) GNMIAddress() string {
	return e.gnmiAddr
}

// GetGNMIState fetches state directly from the in-process server.
func (e *EnvtestEnvironment) GetGNMIState(_ context.Context) ([]byte, error) {
	return e.gnmiServer.GetState()
}

// ClearGNMIState clears state directly on the in-process server.
func (e *EnvtestEnvironment) ClearGNMIState(_ context.Context) error {
	e.gnmiServer.ClearState()
	return nil
}

// PreloadGNMIState replaces the in-process gNMI server state with the given JSON.
// This resets the server to a clean state for test isolation.
func (e *EnvtestEnvironment) PreloadGNMIState(_ context.Context, jsonData []byte) error {
	if len(jsonData) == 0 {
		return nil
	}
	e.gnmiServer.SetState(jsonData)
	return nil
}

// IsEnvtest returns true for envtest mode.
func (e *EnvtestEnvironment) IsEnvtest() bool {
	return true
}

// detectTestBinaryDir locates the envtest binary directory.
// The structure is: bin/k8s/k8s/<version>-<os>-<arch>/
// We need to find the version directory that contains etcd/kube-apiserver.
func detectTestBinaryDir() string {
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
