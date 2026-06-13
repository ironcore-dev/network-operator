// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	prometheusURL  = "https://github.com/prometheus-operator/prometheus-operator/releases/download/v0.82.2/bundle.yaml"
	certmanagerURL = "https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml"
)

// warnError writes a warning to the provided writer.
func warnError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "warning: %v\n", err)
}

// Run executes the provided command within this context.
// It writes the command to the provided writer for logging.
func Run(cmd *exec.Cmd, w io.Writer) (string, error) {
	dir, err := GetProjectDir()
	if err != nil {
		return "", fmt.Errorf("failed to get project directory: %w", err)
	}

	cmd.Dir = dir
	if err = os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(w, "chdir dir: %s\n", err)
	}

	command := strings.Join(cmd.Args, " ")
	// #nosec G705
	_, _ = fmt.Fprintf(w, "running: %s\n", command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed with error: (%w) %s", command, err, string(output))
	}

	return string(output), nil
}

// Apply takes a raw YAML resource and applies it to the cluster by
// creating a temporary file and running 'kubectl apply -f'.
func Apply(ctx context.Context, resource string, w io.Writer) error {
	file, err := os.CreateTemp("", "resource-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	// #nosec G703
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.WriteString(resource); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// #nosec G204 G702
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", file.Name())
	if _, err = Run(cmd, w); err != nil {
		return fmt.Errorf("failed to apply resource: %w", err)
	}
	return nil
}

// ExtractResourceIdentifier parses YAML and returns "kind/name" for use with kubectl wait.
// The kind is lowercased to match kubectl's resource type format.
func ExtractResourceIdentifier(resourceYAML string) (string, error) {
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(resourceYAML), &obj); err != nil {
		return "", fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	kind := strings.ToLower(obj.GetKind())
	name := obj.GetName()
	if kind == "" || name == "" {
		return "", fmt.Errorf("YAML missing kind or metadata.name")
	}

	return kind + "/" + name, nil
}

// CompareJSON compares two JSON strings and returns an error if they are not equal.
// For comparison, it unmarshals both into interface{} and uses reflect.DeepEqual
// after sorting any arrays and removing empty arrays/objects to ignore ordering
// and cleanup artifacts.
func CompareJSON(got, want string) error {
	var gotObj, wantObj any
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		return fmt.Errorf("failed to unmarshal got JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		return fmt.Errorf("failed to unmarshal want JSON: %w", err)
	}

	// Normalize both objects (sort arrays, remove empty containers)
	gotObj = normalizeJSON(gotObj)
	wantObj = normalizeJSON(wantObj)

	if !reflect.DeepEqual(gotObj, wantObj) {
		// For error message, show original compacted JSON (not normalized)
		// so empty objects show as {} not null
		var gotBuf, wantBuf bytes.Buffer
		_ = json.Compact(&gotBuf, []byte(got))
		_ = json.Compact(&wantBuf, []byte(want))
		return fmt.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", gotBuf.String(), wantBuf.String())
	}
	return nil
}

// normalizeJSON recursively sorts arrays and removes empty arrays/objects
// to make comparison order-independent and ignore cleanup artifacts.
func normalizeJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			normalized := normalizeJSON(v)
			// Skip empty maps and empty arrays
			if !isEmpty(normalized) {
				result[k] = normalized
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []any:
		var result []any
		for _, elem := range val {
			normalized := normalizeJSON(elem)
			if !isEmpty(normalized) {
				result = append(result, normalized)
			}
		}
		if len(result) == 0 {
			return nil
		}
		// Sort the array by JSON representation
		sort.Slice(result, func(i, j int) bool {
			bi, _ := json.Marshal(result[i])
			bj, _ := json.Marshal(result[j])
			return string(bi) < string(bj)
		})
		return result
	default:
		return v
	}
}

// isEmpty checks if a value is an empty map, empty array, or nil.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	}
	return false
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "kubectl", "create", "-f", prometheusURL)
	_, err := Run(cmd, w)
	return err
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator(ctx context.Context, w io.Writer) {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", prometheusURL)
	if _, err := Run(cmd, w); err != nil {
		warnError(w, err)
	}
}

// IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// by verifying the existence of key CRDs related to Prometheus.
func IsPrometheusCRDsInstalled(ctx context.Context, w io.Writer) bool {
	// List of common Prometheus CRDs
	prometheusCRDs := []string{
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
	}

	cmd := exec.CommandContext(ctx, "kubectl", "get", "crds", "-o", "custom-columns=NAME:.metadata.name")
	output, err := Run(cmd, w)
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
func InstallCertManager(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", certmanagerURL)
	if _, err := Run(cmd, w); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.CommandContext(
		ctx, "kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)
	if _, err := Run(cmd, w); err != nil {
		return err
	}

	// Wait for webhook to be fully operational (TLS cert ready)
	// The deployment being Available doesn't mean the webhook TLS is ready
	cmd = exec.CommandContext(
		ctx, "kubectl", "wait", "certificate/cert-manager-webhook-ca",
		"--for", "condition=Ready",
		"--namespace", "cert-manager",
		"--timeout", "2m",
	)
	_, _ = Run(cmd, w) // Ignore error - cert may not exist in older versions

	// Give the webhook a moment to pick up the cert
	time.Sleep(5 * time.Second)
	return nil
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager(ctx context.Context, w io.Writer) {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", certmanagerURL)
	if _, err := Run(cmd, w); err != nil {
		warnError(w, err)
	}
}

// WaitForCertManagerWebhook waits for the cert-manager webhook to be fully operational.
// This should be called before deploying resources that use cert-manager certificates.
func WaitForCertManagerWebhook(ctx context.Context, w io.Writer) error {
	// Wait for deployment to be available
	cmd := exec.CommandContext(
		ctx, "kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "2m",
	)
	if _, err := Run(cmd, w); err != nil {
		return err
	}

	// Wait for the CA injector to inject the CA bundle into the webhook
	cmd = exec.CommandContext(
		ctx, "kubectl", "wait", "deployment.apps/cert-manager-cainjector",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "2m",
	)
	if _, err := Run(cmd, w); err != nil {
		return err
	}

	// Wait for the cainjector to inject the CA bundle into the webhook configuration
	// This is what actually makes the webhook work - the API server needs the CA to verify the webhook's TLS cert
	cmd = exec.CommandContext(
		ctx, "kubectl", "wait", "validatingwebhookconfiguration/cert-manager-webhook",
		"--for", "jsonpath={.webhooks[0].clientConfig.caBundle}",
		"--timeout", "2m",
	)
	if _, err := Run(cmd, w); err != nil {
		return fmt.Errorf("cert-manager webhook CA bundle not injected: %w", err)
	}

	return nil
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled(ctx context.Context, w io.Writer) bool {
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
	cmd := exec.CommandContext(ctx, "kubectl", "get", "crds")
	output, err := Run(cmd, w)
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
func LoadImageToKindClusterWithName(ctx context.Context, name string, w io.Writer) error {
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
		// #nosec G703
		defer func() { _ = os.Remove(file.Name()) }()

		// https://github.com/containerd/nerdctl/blob/main/docs/command-reference.md#whale-nerdctl-save
		// https://docs.podman.io/en/v5.3.0/markdown/podman-save.1.html
		// #nosec G702
		cmd := exec.CommandContext(ctx, prov, "save", name, "--output", file.Name())
		if _, err = Run(cmd, w); err != nil {
			return fmt.Errorf("failed to save image: %w", err)
		}

		cmd = exec.CommandContext(ctx, "kind", "load", "image-archive", file.Name(), "--name", cluster) //nolint:gosec
		_, err = Run(cmd, w)
		return err
	}
	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", name, "--name", cluster)
	_, err := Run(cmd, w)
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

// PatchResourceYAML takes a raw YAML resource and patches its namespace and deviceRef.
// This allows txtar test files to have placeholder values that get replaced at runtime.
// It returns the patched YAML string ready for kubectl apply.
func PatchResourceYAML(resourceYAML, namespace, deviceName string) (string, error) {
	// Parse YAML into unstructured map
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(resourceYAML), &obj); err != nil {
		return "", fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Patch metadata.namespace
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		metadata = make(map[string]any)
		obj["metadata"] = metadata
	}
	metadata["namespace"] = namespace

	// Ensure labels map exists and add e2e test label
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		labels = make(map[string]any)
		metadata["labels"] = labels
	}
	labels[E2ETestLabel] = ""

	// Patch spec.deviceRef.name if it exists
	if spec, ok := obj["spec"].(map[string]any); ok {
		if deviceRef, ok := spec["deviceRef"].(map[string]any); ok {
			deviceRef["name"] = deviceName
		}
	}

	// Patch the device label if it exists
	if _, hasDeviceLabel := labels["networking.metal.ironcore.dev/device"]; hasDeviceLabel {
		labels["networking.metal.ironcore.dev/device"] = deviceName
	}

	// Marshal back to YAML
	out, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal patched YAML: %w", err)
	}
	return string(out), nil
}

// ApplyWithPatch applies a YAML resource after patching its namespace and deviceRef.
// This is the cluster-mode equivalent of envtest's createResourceFromTxtar.
func ApplyWithPatch(ctx context.Context, resourceYAML, namespace, deviceName string, w io.Writer) error {
	patched, err := PatchResourceYAML(resourceYAML, namespace, deviceName)
	if err != nil {
		return err
	}
	return Apply(ctx, patched, w)
}

// WaitForCondition waits for a resource to have a condition set to True.
// It tries "Configured" first, falls back to "Ready" if Configured doesn't exist.
// Skips config-only resources that don't have status conditions.
func WaitForCondition(ctx context.Context, resourceName, namespace string, w io.Writer) error {
	// Config-only resources don't have status conditions - skip them
	// resourceName format is "kind/name" e.g. "bgpconfig/evpn-settings"
	kind := strings.Split(resourceName, "/")[0]
	switch strings.ToLower(kind) {
	case "interfaceconfig", "lldpconfig", "bgpconfig", "nveconfig", "managementaccessconfig":
		return nil // No conditions to wait for
	}

	// Try Configured first using jsonpath (more reliable than --for condition=X with multiple conditions)
	cmd := exec.CommandContext(ctx, "kubectl", "wait", resourceName,
		"--for", `jsonpath={.status.conditions[?(@.type=="Configured")].status}=True`,
		"--namespace", namespace,
		"--timeout", "10s",
	)
	if _, err := Run(cmd, w); err == nil {
		return nil
	}

	// Fallback to Ready using jsonpath (condition=Ready doesn't work reliably with custom resources)
	cmd = exec.CommandContext(ctx, "kubectl", "wait", resourceName,
		"--for", `jsonpath={.status.conditions[?(@.type=="Ready")].status}=True`,
		"--namespace", namespace,
		"--timeout", "2m",
	)
	_, err := Run(cmd, w)
	return err
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	before, after, ok := bytes.Cut(content, []byte(target))
	if !ok {
		if bytes.Contains(content, []byte(target)[len(prefix):]) {
			return nil // already uncommented
		}

		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	if _, err = out.Write(before); err != nil {
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

	if _, err = out.Write(after); err != nil {
		return err
	}

	return os.WriteFile(filename, out.Bytes(), 0o644)
}

// ExtractConditions extracts status conditions from an unstructured object
// into a typed []metav1.Condition slice for use with apimeta helpers.
func ExtractConditions(obj *unstructured.Unstructured) ([]metav1.Condition, error) {
	raw, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var conditions []metav1.Condition
	return conditions, json.Unmarshal(data, &conditions)
}
