// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var (
	// gnmiPort is the port on which the gnmi-test-server listens for gNMI requests.
	gnmiPort uint16 = 9339
	// serverImage is the container image for the gnmi-test-server.
	// This must match the image built by the Makefile.
	serverImage = "ghcr.io/ironcore-dev/gnmi-test-server:latest"
)

// ClusterEnvironment enables end-to-end tests to run against a real Kubernetes cluster (e.g., Kind).
// TODO: use native library instead of kubectl (follow up)
type ClusterEnvironment struct {
	restConfig *rest.Config
	k8sClient  client.Client
}

// NewClusterEnvironment creates a new cluster-based test environment.
func NewClusterEnvironment() *ClusterEnvironment {
	return &ClusterEnvironment{}
}

// Setup connects to the existing cluster (CRDs should already be installed).
func (c *ClusterEnvironment) Setup(ctx context.Context) error {
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

	// Get REST config from kubeconfig
	var err error
	c.restConfig = ctrl.GetConfigOrDie()

	c.k8sClient, err = client.New(c.restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	return nil
}

// InstallCRDs installs CRDs into the cluster. Should only be called once (from process 1).
// Uses server-side apply to handle existing CRDs gracefully.
func (c *ClusterEnvironment) InstallCRDs(ctx context.Context) error {
	if err := c.runKubectl(ctx, "apply", "-k", "config/crd", "--server-side", "--force-conflicts"); err != nil {
		return fmt.Errorf("failed to install CRDs: %w", err)
	}
	return nil
}

// DeployManager deploys the controller-manager and waits for it to be ready.
// Should only be called once (from process 1).
// Respects E2E_PROVIDER env var for provider selection.
func (c *ClusterEnvironment) DeployManager(ctx context.Context) error {
	dir, _ := GetProjectDir() //nolint:errcheck // uses current dir as fallback
	env := os.Environ()
	if provider := os.Getenv("E2E_PROVIDER"); provider != "" {
		env = append(env, "PROVIDER="+provider)
	}

	// First deploy CRDs explicitly (make deploy also does this, but let's be sure)
	cmd := exec.CommandContext(ctx, "make", "deploy-crds")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = nil // Prevent stdin inheritance that can cause hangs
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to deploy CRDs: %s: %w", string(output), err)
	}

	// Then deploy the manager
	cmd = exec.CommandContext(ctx, "make", "deploy")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = nil // Prevent stdin inheritance that can cause hangs
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to deploy manager: %s: %w", string(output), err)
	}

	if err := c.runKubectl(ctx, "wait", "deployment/network-operator-controller-manager",
		"-n", "network-operator-system",
		"--for", "condition=Available",
		"--timeout", "2m"); err != nil {
		return fmt.Errorf("manager not ready: %w", err)
	}
	return nil
}

// UndeployManager undeploys the controller-manager.
// Uses kubectl delete -k directly instead of make undeploy to avoid shell pipeline stdin issues.
func (c *ClusterEnvironment) UndeployManager(ctx context.Context) error {
	if err := c.runKubectl(ctx, "delete", "-k", "config/develop", "--ignore-not-found"); err != nil {
		return fmt.Errorf("failed to undeploy manager: %w", err)
	}
	return nil
}

// Teardown cleans up CRDs.
func (c *ClusterEnvironment) Teardown(ctx context.Context) error {
	_ = c.runKubectl(ctx, "delete", "-k", "config/crd", "--ignore-not-found") //nolint:errcheck // best-effort cleanup
	return nil
}

// Client returns the Kubernetes client.
func (c *ClusterEnvironment) Client() client.Client {
	return c.k8sClient
}

// RESTConfig returns the REST config.
func (c *ClusterEnvironment) RESTConfig() *rest.Config {
	return c.restConfig
}

// DeployGNMIServer deploys the gnmi-test-server pod and returns its gNMI address.
func (c *ClusterEnvironment) DeployGNMIServer(ctx context.Context, namespace string) (netip.AddrPort, error) {
	if err := c.runKubectl(
		ctx,
		"run", "gnmi-test-server",
		"--image", serverImage,
		"--image-pull-policy", "Never",
		"--namespace", namespace,
		"--restart", "Never",
		"--port", "8000",
		"--port", strconv.FormatUint(uint64(gnmiPort), 10),
	); err != nil {
		return netip.AddrPort{}, fmt.Errorf("failed to deploy gnmi-test-server: %w", err)
	}

	if err := c.runKubectl(
		ctx,
		"wait", "pods/gnmi-test-server",
		"--for", "condition=Ready",
		"--namespace", namespace,
		"--timeout", "1m",
	); err != nil {
		return netip.AddrPort{}, fmt.Errorf("gnmi-test-server pod not ready: %w", err)
	}

	out, err := c.runKubectlOutput(
		ctx,
		"get", "pod", "gnmi-test-server",
		"--output", "jsonpath={.status.podIP}",
		"--namespace", namespace,
	)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("failed to get gnmi-test-server IP: %w", err)
	}
	var s netip.Addr
	if s, err = netip.ParseAddr(strings.TrimSpace(out)); err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid IP address from gnmi-test-server pod: %w", err)
	}

	return netip.AddrPortFrom(s, gnmiPort), nil
}

// GetGNMIState fetches state via kubectl exec.
func (c *ClusterEnvironment) GetGNMIState(ctx context.Context, namespace string) ([]byte, error) {
	out, err := c.runKubectlOutput(
		ctx,
		"exec", "gnmi-test-server",
		"--namespace", namespace,
		"--",
		"wget", "-qO-", "http://localhost:8000/v1/state",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get gNMI state: %w", err)
	}
	return []byte(out), nil
}

// ClearGNMIState clears state.
func (c *ClusterEnvironment) ClearGNMIState(ctx context.Context, namespace string) error {
	_, err := c.runKubectlOutput(
		ctx,
		"exec", "gnmi-test-server",
		"--namespace", namespace,
		"--",
		"wget", "-qO-", "--post-data=", "http://localhost:8000/v1/clear",
	)
	return err
}

// PreloadGNMIState preloads nested JSON into the gnmi-test-server state.
// This allows tests to set up paths like System/procsys-items/bootTime
// before the Device controller reconciles.
func (c *ClusterEnvironment) PreloadGNMIState(ctx context.Context, namespace string, jsonData []byte) error {
	_, err := c.runKubectlOutput(
		ctx,
		"exec", "gnmi-test-server",
		"--namespace", namespace,
		"--",
		"wget", "-qO-", "--post-data="+string(jsonData), "http://localhost:8000/v1/state",
	)
	return err
}

// runKubectl runs a kubectl command and returns an error if it fails.
func (c *ClusterEnvironment) runKubectl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	dir, _ := GetProjectDir() //nolint:errcheck // uses current dir as fallback
	cmd.Dir = dir
	cmd.Stdin = nil // Prevent stdin inheritance that can cause hangs
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}

// runKubectlOutput runs a kubectl command and returns its output.
func (c *ClusterEnvironment) runKubectlOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	dir, _ := GetProjectDir() //nolint:errcheck // uses current dir as fallback
	cmd.Dir = dir
	cmd.Stdin = nil // Prevent stdin inheritance that can cause hangs
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return stdout.String(), nil
}

// DeleteNamespace deletes the given namespace.
func (c *ClusterEnvironment) DeleteNamespace(ctx context.Context, namespace string) error {
	if err := c.runKubectl(ctx, "delete", "namespace", namespace, "--ignore-not-found"); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", namespace, err)
	}
	return nil
}

// CreateNamespace creates a new namespace with the given name and labels it for cleanup tracking.
func (c *ClusterEnvironment) CreateNamespace(ctx context.Context, namespace string) error {
	if err := c.runKubectl(ctx, "create", "namespace", namespace); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}
	// Label the namespace for cleanup tracking across parallel test processes
	if err := c.runKubectl(ctx, "label", "namespace", namespace, E2ETestLabel+"="); err != nil {
		return fmt.Errorf("failed to label namespace %s: %w", namespace, err)
	}
	return nil
}

// WaitForTestNamespacesGone waits for all labeled test namespaces to be fully deleted.
// This ensures DeferCleanup hooks have completed before the manager is undeployed.
// Without this, the manager (and CRDs) may be deleted while resources with finalizers still exist,
// leaving them stuck forever because the controller can no longer process the finalizers.
func (c *ClusterEnvironment) WaitForTestNamespacesGone(ctx context.Context) error {
	// Get all namespaces with our e2e test label
	out, err := c.runKubectlOutput(
		ctx,
		"get", "namespaces",
		"-l", E2ETestLabel,
		"-o", "jsonpath={.items[*].metadata.name}",
	)
	if err != nil {
		return fmt.Errorf("failed to list test namespaces: %w", err)
	}

	namespaces := strings.Fields(out)
	if len(namespaces) == 0 {
		return nil
	}

	// Wait for each test namespace to be deleted (with timeout)
	for _, ns := range namespaces {
		_ = c.runKubectl(ctx, "wait", "namespace", ns, //nolint:errcheck // best-effort cleanup
			"--for=delete",
			"--timeout=120s")
	}

	return nil
}

// DeleteCustomResources deletes all custom resources in the given namespace.
// Deletion order: CoreResources first (have finalizers), then ConfigResources, then Device.
// This allows finalizers to complete while their dependencies still exist.
func (c *ClusterEnvironment) DeleteCustomResources(ctx context.Context, namespace string) error {
	deleteResources := func(gvks []schema.GroupVersionKind) {
		for _, gvk := range gvks {
			_ = c.runKubectl(ctx, "delete", //nolint:errcheck // best-effort cleanup
				ResourcePluralName(gvk), "--all",
				"--namespace", namespace,
				"--ignore-not-found")
		}
		// Wait for resources to be fully gone (finalizers completed)
		for _, gvk := range gvks {
			_ = c.runKubectl(ctx, "wait", //nolint:errcheck // best-effort cleanup
				ResourcePluralName(gvk),
				"--for=delete", "--all",
				"--namespace", namespace,
				"--timeout=60s")
		}
	}

	// Delete core resources first (have finalizers that need Device + configs)
	deleteResources(CoreResources)
	// Then delete config resources
	deleteResources(ConfigResources)

	// Delete Device LAST - after all other resources and their finalizers are done
	_ = c.runKubectl(ctx, "delete", "devices", "--all", //nolint:errcheck // best-effort cleanup
		"--namespace", namespace,
		"--ignore-not-found")
	_ = c.runKubectl(ctx, "wait", "devices", //nolint:errcheck // best-effort cleanup
		"--for=delete", "--all",
		"--namespace", namespace,
		"--timeout=60s")

	return nil
}
