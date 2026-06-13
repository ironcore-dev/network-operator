// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build !envtest

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/txtar"

	"github.com/ironcore-dev/network-operator/test/e2e/testutil"
)

// namespace where the project is deployed in
// tests create resources in separate namespaces
const namespace = "network-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "network-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "network-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "network-operator-metrics-binding"

// testEnv is the cluster test environment.
var testEnv *testutil.ClusterEnvironment

func init() {
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting network-operator tests in CLUSTER mode\n")
}

// Manager Setup tests run serially on a single Ginkgo process.
// These tests deploy and verify the controller-manager before reconciliation tests run in parallel.
var _ = Describe("Manager Setup", Serial, Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func(ctx SpecContext) {
		By("creating manager namespace")
		cmd := exec.CommandContext(ctx, "kubectl", "create", "ns", namespace, "--dry-run=client", "-o", "yaml")
		nsYaml, err := testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate namespace YAML")
		cmd = exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
		cmd.Stdin = bytes.NewBufferString(nsYaml)
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.CommandContext(ctx, "kubectl", "label", "--overwrite", "ns", namespace, "pod-security.kubernetes.io/enforce=restricted")
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.CommandContext(ctx, "make", "deploy-crds")
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.CommandContext(ctx, "make", "deploy")
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all setup tests complete, clean up the manager.
	// Note: CRDs are left installed for the parallel reconciliation tests.
	AfterAll(func(ctx SpecContext) {
		By("cleaning up the ClusterRoleBinding of the service account to allow access to metrics")
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
		_, err := testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete ClusterRoleBinding")

		By("cleaning up the curl pod for metrics")
		cmd = exec.CommandContext(ctx, "kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete curl-metrics pod")
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func(ctx SpecContext) {
		if specReport := CurrentSpecReport(); specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.CommandContext(ctx, "kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := testutil.Run(cmd, GinkgoWriter)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.CommandContext(ctx, "kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := testutil.Run(cmd, GinkgoWriter)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.CommandContext(ctx, "kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := testutil.Run(cmd, GinkgoWriter)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.CommandContext(ctx, "kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := testutil.Run(cmd, GinkgoWriter)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	It("should run successfully", func(ctx SpecContext) {
		By("validating that the controller-manager pod is running as expected")
		verifyControllerUp := func(g Gomega) {
			// Get the name of the controller-manager pod
			cmd := exec.CommandContext(
				ctx, "kubectl", "get",
				"pods", "-l", "control-plane=controller-manager",
				"-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace,
			)

			podOutput, err := testutil.Run(cmd, GinkgoWriter)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
			podNames := testutil.GetNonEmptyLines(podOutput)
			g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
			controllerPodName = podNames[0]
			g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

			// Validate the pod's status
			cmd = exec.CommandContext(ctx, "kubectl", "get", "pods", controllerPodName, "-o", "jsonpath={.status.phase}", "-n", namespace)
			output, err := testutil.Run(cmd, GinkgoWriter)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
		}
		Eventually(verifyControllerUp).Should(Succeed())
	})

	It("should ensure the metrics endpoint is serving metrics", func(ctx SpecContext) {
		By("creating a ClusterRoleBinding for the service account to allow access to metrics")
		// #nosec G204
		cmd := exec.CommandContext(ctx, "kubectl", "create", "clusterrolebinding", metricsRoleBindingName, "--clusterrole=network-operator-metrics-reader", fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName))
		_, err := testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

		By("validating that the metrics service is available")
		cmd = exec.CommandContext(ctx, "kubectl", "get", "service", metricsServiceName, "-n", namespace)
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

		By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
		cmd = exec.CommandContext(ctx, "kubectl", "get", "ServiceMonitor", "-n", namespace)
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

		By("getting the service account token")
		token, err := serviceAccountToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(token).NotTo(BeEmpty())

		By("waiting for the metrics endpoint to be ready")
		verifyMetricsEndpointReady := func(g Gomega) {
			kcmd := exec.CommandContext(ctx, "kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
			output, kErr := testutil.Run(kcmd, GinkgoWriter)
			g.Expect(kErr).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
		}
		Eventually(verifyMetricsEndpointReady).Should(Succeed())

		By("verifying that the controller manager has started")
		verifyManagerStarted := func(g Gomega) {
			kcmd := exec.CommandContext(ctx, "kubectl", "logs", controllerPodName, "-n", namespace)
			output, kErr := testutil.Run(kcmd, GinkgoWriter)
			g.Expect(kErr).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("starting manager"), "Manager not yet started")
		}
		Eventually(verifyManagerStarted).Should(Succeed())

		By("creating the curl-metrics pod to access the metrics endpoint")
		// #nosec G204
		cmd = exec.CommandContext(ctx, "kubectl", "run", "curl-metrics", "--restart=Never",
			"--namespace", namespace,
			"--image=curlimages/curl:latest",
			"--overrides",
			fmt.Sprintf(`{
				"spec": {
					"containers": [{
						"name": "curl",
						"image": "curlimages/curl:latest",
						"command": ["/bin/sh", "-c"],
						"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": ["ALL"]
							},
							"runAsNonRoot": true,
							"runAsUser": 1000,
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}],
					"serviceAccount": "%s"
				}
			}`, token, metricsServiceName, namespace, serviceAccountName))
		_, err = testutil.Run(cmd, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

		By("waiting for the curl-metrics pod to complete.")
		verifyCurlUp := func(g Gomega) {
			cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "curl-metrics", "-o", "jsonpath={.status.phase}", "-n", namespace)
			output, err := testutil.Run(cmd, GinkgoWriter)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
		}
		Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

		By("getting the metrics by checking curl-metrics logs")
		metricsOutput := getMetricsOutput(ctx)
		Expect(metricsOutput).To(ContainSubstring("controller_runtime_webhook_panics_total"))
	})

	It("should provisioned cert-manager", func(ctx SpecContext) {
		By("validating that cert-manager has the certificate Secret")
		verifyCertManager := func(g Gomega) {
			cmd := exec.CommandContext(ctx, "kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
			_, err := testutil.Run(cmd, GinkgoWriter)
			g.Expect(err).NotTo(HaveOccurred())
		}
		Eventually(verifyCertManager).Should(Succeed())
	})

	It("should have CA injection for validating webhooks", func(ctx SpecContext) {
		By("checking CA injection for validating webhooks")
		verifyCAInjection := func(g Gomega) {
			cmd := exec.CommandContext(ctx, "kubectl", "get",
				"validatingwebhookconfigurations.admissionregistration.k8s.io",
				"network-operator-validating-webhook-configuration",
				"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
			vwhOutput, err := testutil.Run(cmd, GinkgoWriter)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(vwhOutput)).To(BeNumerically(">", 10))
		}
		Eventually(verifyCAInjection).Should(Succeed())
	})

	// +kubebuilder:scaffold:e2e-webhooks-checks
})

// Reconciliation tests run in parallel across multiple Ginkgo processes.
// Each test creates its own namespace and gnmi-test-server instance for isolation.
var _ = Describe("Reconciliation", func() {
	projectDir, err := testutil.GetProjectDir()
	if err != nil {
		Fail(fmt.Sprintf("Failed to get project directory: %v", err))
	}

	// Get provider filter from environment (set by Makefile)
	providerFilter := os.Getenv("E2E_PROVIDER")

	testdataRoot := filepath.Join(projectDir, "test", "e2e", "testdata")
	providerDirs, err := os.ReadDir(testdataRoot)
	if err != nil {
		Fail(fmt.Sprintf("Failed to read testdata directory: %v", err))
	}

	var testFiles []string
	var providerName string
	for _, providerDir := range providerDirs {
		if !providerDir.IsDir() {
			continue
		}
		providerName = providerDir.Name()

		if providerFilter != "" && providerName != providerFilter {
			continue
		}

		providerTestdataDir := filepath.Join(testdataRoot, providerName)

		testFiles, err = filepath.Glob(filepath.Join(providerTestdataDir, "*.txt"))
		if err != nil {
			Fail(fmt.Sprintf("Failed to glob testdata: %v", err))
		}
		break
	}

	for _, testFile := range testFiles {
		testName := filepath.Base(testFile)
		testName = testName[:len(testName)-4] // remove .txt

		It(fmt.Sprintf("should reconcile %s/%s", providerName, testName), func(ctx SpecContext) {
			By("parsing testdata file")
			a, err := txtar.ParseFile(testFile)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse test file: %s", testFile)

			var state, preload []byte
			var resources []txtar.File
			for _, f := range a.Files {
				switch f.Name {
				case "state/expect":
					state = f.Data
				case "state/preload":
					preload = f.Data
				default:
					resources = append(resources, f)
				}
			}
			Expect(state).NotTo(BeEmpty(), "Expected '-- state/expect --' section in testdata")
			Expect(resources).NotTo(BeEmpty(), "Expected at least one resource in testdata")

			By("creating test namespace")
			testNamespace := fmt.Sprintf("test-%s-%s-%s", providerName, strings.ReplaceAll(testName, "_", "-"), time.Now().Format("20060102150405"))
			// Truncate to 63 chars max (K8s namespace limit)
			if len(testNamespace) > 63 {
				testNamespace = testNamespace[:63]
			}
			Expect(testEnv.CreateNamespace(ctx, testNamespace)).NotTo(HaveOccurred(), "Failed to create test namespace")

			DeferCleanup(func(ctx SpecContext) {
				// Clean up test resources before deleting the gnmi-test-server pod to avoid issues with finalizers that require API access.
				By("deleting test resources")
				testEnv.DeleteCustomResources(ctx, testNamespace)
				By("deleting test namespace")
				testEnv.DeleteNamespace(ctx, testNamespace)
			})

			deviceName := fmt.Sprintf("test-device-%d", time.Now().UnixNano())

			By("deploying a gnmi-test-server instance for this test")
			gnmiAddr, err := testEnv.DeployGNMIServer(ctx, testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to deploy gnmi-test-server")
			Expect(gnmiAddr).ToNot(BeNil())

			By("preloading gNMI state if specified")
			if len(preload) > 0 {
				err = testEnv.PreloadGNMIState(ctx, testNamespace, preload)
				Expect(err).NotTo(HaveOccurred(), "Failed to preload gNMI state")
			}

			By("creating a test device")
			device := fmt.Sprintf(`
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: Device
metadata:
  name: %s
  namespace: %s
  labels:
    %s: ""
spec:
  endpoint:
    address: "%s"`, deviceName, testNamespace, testutil.E2ETestLabel, gnmiAddr.String())
			err = testutil.Apply(ctx, device, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply Device")

			By("applying resources from testdata")
			_, _ = fmt.Fprintf(GinkgoWriter, "DEBUG: Found %d resources to apply\n", len(resources))
			for _, res := range resources {
				_, _ = fmt.Fprintf(GinkgoWriter, "DEBUG: Applying resource: %s\n", res.Name)
				err = testutil.ApplyWithPatch(ctx, string(res.Data), testNamespace, deviceName, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred(), "Failed to apply resource: %s", res.Name)
			}

			By("waiting for resources to be configured")
			for _, res := range resources {
				// Extract actual kind/name from YAML since section name may differ from metadata.name
				resourceID, err := testutil.ExtractResourceIdentifier(string(res.Data))
				Expect(err).NotTo(HaveOccurred(), "Failed to extract resource identifier from: %s", res.Name)
				err = testutil.WaitForCondition(ctx, resourceID, testNamespace, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred(), "Resource not configured: %s", resourceID)
			}

			By("verifying gNMI state matches expected JSON")
			gnmiState, err := testEnv.GetGNMIState(ctx, testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to get gNMI state")

			err = testutil.CompareJSON(string(gnmiState), string(state))
			Expect(err).NotTo(HaveOccurred(), "gNMI state does not match expected JSON")
		})
	}
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken(ctx context.Context) (string, error) {
	// #nosec G101
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := serviceAccountName + "-token-request"
	tokenRequestFile := filepath.Join(os.TempDir(), secretName)
	if err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644)); err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		// #nosec G204
		cmd := exec.CommandContext(ctx, "kubectl", "create", "--raw", fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/%s/token", namespace, serviceAccountName), "-f", tokenRequestFile)
		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, nil
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput(ctx context.Context) string {
	By("getting the curl-metrics logs")
	cmd := exec.CommandContext(ctx, "kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := testutil.Run(cmd, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
