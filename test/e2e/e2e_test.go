// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
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
)

// namespace where the project is deployed in
const namespace = "network-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "network-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "network-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "network-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string
	var gnmiServerIPAddr string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("deploying the gnmi-test-server")
		cmd := exec.Command("kubectl", "run", "gnmi-test-server",
			"--image", serverImage,
			"--image-pull-policy", "Never",
			"--namespace", "default",
			"--restart", "Never",
			"--port", "8000",
			"--port", "9339",
		)
		_, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the gnmi-test-server")

		cmd = exec.Command("kubectl", "wait", "pods/gnmi-test-server",
			"--for", "condition=Ready",
			"--namespace", "default",
			"--timeout", "1m",
		)
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command("kubectl", "get", "pod", "gnmi-test-server",
			"--output", "jsonpath='{.status.podIP}'",
			"--namespace", "default",
		)
		out, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		gnmiServerIPAddr = strings.ReplaceAll(strings.TrimSpace(out), "'", "")

		By("creating manager namespace")
		cmd = exec.Command("kubectl", "create", "ns", namespace)
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace, "pod-security.kubernetes.io/enforce=restricted")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "deploy-crds")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", "IMG="+image)
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the ClusterRoleBinding of the service account to allow access to metrics")
		cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName)
		_, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete ClusterRoleBinding")

		By("cleaning up the curl pod for metrics")
		cmd = exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete curl-metrics pod")

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to undeploy the controller-manager")

		By("uninstalling CRDs")
		cmd = exec.Command("make", "undeploy-crds")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to uninstall CRDs")

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace")

		By("cleaning up the gnmi-test-server pod")
		cmd = exec.Command("kubectl", "delete", "pod", "gnmi-test-server", "-n", "default")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete gnmi-test-server pod")
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		if specReport := CurrentSpecReport(); specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get", "pods", controllerPodName, "-o", "jsonpath={.status.phase}", "-n", namespace)
				output, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			// #nosec G204
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName, "--clusterrole=network-operator-metrics-reader", fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName))
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
			cmd = exec.Command("kubectl", "get", "ServiceMonitor", "-n", namespace)
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				kcmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, kErr := Run(kcmd)
				g.Expect(kErr).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				kcmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, kErr := Run(kcmd)
				g.Expect(kErr).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"), "Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			// #nosec G204
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
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
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics", "-o", "jsonpath={.status.phase}", "-n", namespace)
				output, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring("controller_runtime_webhook_panics_total"))
		})

		It("should provisioned cert-manager", func() {
			By("validating that cert-manager has the certificate Secret")
			verifyCertManager := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
				_, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyCertManager).Should(Succeed())
		})

		It("should have CA injection for validating webhooks", func() {
			By("checking CA injection for validating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"validatingwebhookconfigurations.admissionregistration.k8s.io",
					"network-operator-validating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				vwhOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(vwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))

		DescribeTable("Should reconcile the api objects",
			func(file string) {
				device := `
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: Device
metadata:
  name: device
  namespace: default
spec:
  endpoint:
    address: "%s"`
				err := Apply(fmt.Sprintf(device, gnmiServerIPAddr+":9339"))
				Expect(err).NotTo(HaveOccurred(), "Failed to apply Device")

				dir, err := GetProjectDir()
				Expect(err).NotTo(HaveOccurred(), "Failed to get project directory")

				a, err := txtar.ParseFile(filepath.Join(dir, "test", "e2e", "testdata", file))
				Expect(err).NotTo(HaveOccurred(), "Failed to parse test file")
				Expect(a.Files).To(HaveLen(2), "Expected 2 files in the test archive")

				err = Apply(string(a.Files[0].Data))
				Expect(err).NotTo(HaveOccurred(), "Failed to apply Interface")

				// #nosec G204
				cmd := exec.Command("kubectl", "wait", a.Files[0].Name,
					"--for", "condition=Configured",
					"--namespace", "default",
					"--timeout", "5m",
				)
				_, err = Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "exec", "gnmi-test-server",
					"--namespace", "default",
					"--",
					"wget", "-qO-", "http://localhost:8000/v1/state",
				)
				got, err := Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to execute command on gnmi-test-server")

				err = CompareJSON(got, string(a.Files[1].Data))
				Expect(err).NotTo(HaveOccurred(), "State output does not match expected JSON")

				// #nosec G204
				cmd = exec.Command("kubectl", "delete", a.Files[0].Name)
				_, err = Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to delete object")

				cmd = exec.Command("kubectl", "delete", "devices/device")
				_, err = Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to delete object")

				cmd = exec.Command("kubectl", "exec", "gnmi-test-server",
					"--namespace", "default",
					"--",
					"wget", "-qO-", "--header='X-HTTP-Method-Override: DELETE'", "http://localhost:8000/v1/state",
				)
				_, err = Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to execute command on gnmi-test-server")
			},
			Entry("Loopback Interface", "interface.txt"),
		)
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	// #nosec G101
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := serviceAccountName + "-token-request"
	tokenRequestFile := filepath.Join("/tmp", secretName)
	if err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644)); err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		// #nosec G204
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/%s/token", namespace, serviceAccountName), "-f", tokenRequestFile)
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
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := Run(cmd)
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
