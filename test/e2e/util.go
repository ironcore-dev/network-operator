// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive
)

const (
	prometheusURL  = "https://github.com/prometheus-operator/prometheus-operator/releases/download/v0.82.2/bundle.yaml"
	certmanagerURL = "https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, err := GetProjectDir()
	if err != nil {
		return "", fmt.Errorf("failed to get project directory: %w", err)
	}

	cmd.Dir = dir
	if err = os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed with error: (%w) %s", command, err, string(output))
	}

	return string(output), nil
}

// Apply takes a raw YAML resource and applies it to the cluster by
// creating a temporary file and running 'kubectl apply -f'.
func Apply(resource string) error {
	file, err := os.CreateTemp("", "resource-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write([]byte(resource)); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// #nosec G204
	cmd := exec.Command("kubectl", "apply", "-f", file.Name())
	if _, err = Run(cmd); err != nil {
		return fmt.Errorf("failed to apply resource: %w", err)
	}
	return nil
}

// CompareJSON compares two JSON strings and returns an error if they are not equal.
// For comparison, it will compact the JSON strings to remove any whitespace differences.
// If the JSON strings are equal, it returns nil.
func CompareJSON(got, want string) error {
	var gotBuf, wantBuf bytes.Buffer
	if err := json.Compact(&gotBuf, []byte(got)); err != nil {
		return fmt.Errorf("failed to compact got JSON: %w", err)
	}
	if err := json.Compact(&wantBuf, []byte(want)); err != nil {
		return fmt.Errorf("failed to compact want JSON: %w", err)
	}

	if !bytes.Equal(gotBuf.Bytes(), wantBuf.Bytes()) {
		return fmt.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", gotBuf.String(), wantBuf.String())
	}
	return nil
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	cmd := exec.Command("kubectl", "create", "-f", prometheusURL)
	_, err := Run(cmd)
	return err
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	cmd := exec.Command("kubectl", "delete", "-f", prometheusURL)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// by verifying the existence of key CRDs related to Prometheus.
func IsPrometheusCRDsInstalled() bool {
	// List of common Prometheus CRDs
	prometheusCRDs := []string{
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
	}

	cmd := exec.Command("kubectl", "get", "crds", "-o", "custom-columns=NAME:.metadata.name")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	crdList := GetNonEmptyLines(output)
	for _, crd := range prometheusCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	cmd := exec.Command("kubectl", "apply", "-f", certmanagerURL)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	cmd := exec.Command("kubectl", "delete", "-f", certmanagerURL)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled() bool {
	// List of common Cert Manager CRDs
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"certificaterequests.cert-manager.io",
		"orders.acme.cert-manager.io",
		"challenges.acme.cert-manager.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range certManagerCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	// See: https://kind.sigs.k8s.io/docs/user/rootless/#creating-a-kind-cluster-with-rootless-nerdctl
	prov, ok := os.LookupEnv("KIND_EXPERIMENTAL_PROVIDER")
	if ok && prov != "docker" {
		// If kind is configured to not use the docker runtime (e.g. when using podman or nerctl),
		// we need to create a temp file to store the image archive and load it as a tarball.
		// See: https://github.com/kubernetes-sigs/kind/issues/2760
		file, err := os.CreateTemp("", "operator-image-")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		_ = file.Close()
		defer func() { _ = os.Remove(file.Name()) }()

		// https://github.com/containerd/nerdctl/blob/main/docs/command-reference.md#whale-nerdctl-save
		// https://docs.podman.io/en/v5.3.0/markdown/podman-save.1.html
		cmd := exec.Command(prov, "save", name, "--output", file.Name())
		if _, err = Run(cmd); err != nil {
			return fmt.Errorf("failed to save image: %w", err)
		}

		cmd = exec.Command("kind", "load", "image-archive", file.Name(), "--name", cluster) //nolint:gosec
		_, err = Run(cmd)
		return err
	}
	cmd := exec.Command("kind", "load", "docker-image", name, "--name", cluster)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	for element := range strings.SplitSeq(output, "\n") {
		if element != "" {
			res = append(res, element)
		}
	}
	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	idx := strings.Index(string(content), target)
	if idx < 0 {
		if strings.Contains(string(content), target[len(prefix):]) {
			return nil // already uncommented
		}

		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	if _, err = out.Write(content[:idx]); err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		_, err = out.WriteString(strings.TrimPrefix(scanner.Text(), prefix))
		if err != nil {
			return err
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err = out.WriteString("\n"); err != nil {
			return err
		}
	}

	if _, err = out.Write(content[idx+len(target):]); err != nil {
		return err
	}

	return os.WriteFile(filename, out.Bytes(), 0o644)
}
