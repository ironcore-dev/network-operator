// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package main_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"rsc.io/script"
	"rsc.io/script/scripttest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

const (
	timeout  = 10 * time.Second
	interval = time.Second
)

// TestAll runs all lab tests by setting up the environment, SSH connection,
// and Kubernetes client, then executing all test cases from the testdata directory.
func TestAll(t *testing.T) {
	ReadEnv(t)
	SetupSSH(t)
	SetupK8s(t)
	engine := &script.Engine{
		Conds: DefaultConds(),
		Cmds:  DefaultCmds(),
		Quiet: !testing.Verbose(),
	}
	env := os.Environ()
	scripttest.Test(t, t.Context(), engine, env, "testdata/*.txt")
}

// DefaultCmds returns a set of script commands.
//
// This set includes all of the commands in [scripttest.DefaultConds].
func DefaultCmds() map[string]script.Cmd {
	cmds := scripttest.DefaultCmds()
	cmds["vty"] = Vty()
	cmds["apply"] = Apply()
	return cmds
}

// DefaultConds returns a set of script conditions.
//
// This set includes all of the conditions in [scripttest.DefaultConds].
func DefaultConds() map[string]script.Cond {
	conds := scripttest.DefaultConds()
	return conds
}

// Vty returns a script command that executes commands on the target network device
// through an SSH virtual terminal session.
func Vty() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "run a command through the virtual terminal on the device",
			Args:    "command [args...]",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, script.ErrUsage
			}
			return func(*script.State) (stdout, stderr string, reterr error) {
				session, err := sshClient.NewSession()
				if err != nil {
					return "", "", fmt.Errorf("failed to create ssh session: %w", err)
				}
				defer func() {
					if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
						reterr = errors.Join(reterr, fmt.Errorf("failed to close ssh session: %w", err))
					}
				}()
				var stdoutBuf, stderrBuf bytes.Buffer
				session.Stdout = &stdoutBuf
				session.Stderr = &stderrBuf
				reterr = session.Run(strings.Join(args, " "))
				stdout = stdoutBuf.String()
				stderr = stderrBuf.String()
				return
			}, nil
		})
}

// Apply returns a script command that applies a Kubernetes manifest to the cluster
// and waits for the resource to reach a Ready state with automatic timeout handling.
func Apply() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "apply a Kubernetes manifest to the cluster",
			Args:    "file",
			Detail: []string{
				"Note that this command does not wait for the resources to be ready.",
				"Use `wait` to wait for resources to be ready after applying.",
			},
			Async: true,
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 1 {
				return nil, script.ErrUsage
			}
			data, err := os.ReadFile(s.Path(args[0]))
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", args[0], err)
			}
			json, err := yaml.YAMLToJSON(data)
			if err != nil {
				return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
			dec := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()
			obj, _, err := dec.Decode(json, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to decode resource: %w", err)
			}
			res, ok := obj.(client.Object)
			if !ok {
				return nil, fmt.Errorf("decoded object is not a client.Object: %T", obj)
			}
			res.SetNamespace(metav1.NamespaceDefault)
			res.SetLabels(map[string]string{v1alpha1.DeviceLabel: "device"})
			if err := k8sClient.Create(s.Context(), res); err != nil {
				return nil, fmt.Errorf("failed to apply resource: %w", err)
			}
			wait := func(s *script.State) (stdout, stderr string, reterr error) {
				if err := k8sClient.Get(s.Context(), client.ObjectKeyFromObject(res), res); err != nil {
					return "", "", fmt.Errorf("failed to get resource: %w", err)
				}
				ready, err := IsObjectReady(res)
				if err != nil || ready {
					return "", "", err
				}
				return "", "", fmt.Errorf("resource %s is not ready", res.GetName())
			}
			return WaitTimeout(wait, timeout, interval), nil
		})
}

// TODO(felix-kaestner): Load endpoint configuration from a config file.
var Endpoint = struct {
	Addr string
	User string
	Pass string // #nosec G117
}{}

// ReadEnv reads required environment variables and populates the global Endpoint struct.
func ReadEnv(t *testing.T) {
	Endpoint.Addr = MustGetEnv(t, "ADDR")
	Endpoint.User = MustGetEnv(t, "USER")
	Endpoint.Pass = MustGetEnv(t, "PASS")
}

var sshClient *ssh.Client

// SetupSSH establishes an SSH connection to the target network device using
// the credentials from the global Endpoint struct and registers cleanup.
func SetupSSH(t *testing.T) {
	t.Helper()
	var err error
	sshClient, err = ssh.Dial("tcp", net.JoinHostPort(Endpoint.Addr, "22"), &ssh.ClientConfig{
		User:            Endpoint.User,
		Auth:            []ssh.AuthMethod{ssh.Password(Endpoint.Pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() {
		if err := sshClient.Close(); err != nil {
			t.Fatalf("failed to close SSH client: %v", err)
		}
	})
}

var k8sClient client.Client

// SetupK8s initializes the Kubernetes client and creates the necessary resources
// (Secret for credentials and Device) required for the lab tests.
func SetupK8s(t *testing.T) {
	t.Helper()
	config, err := ctrlconfig.GetConfigWithContext(os.Getenv("KUBECONTEXT"))
	if err != nil {
		t.Fatalf("failed to get Kubernetes config: %v", err)
	}
	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("failed to add v1alpha1 scheme: %v", err)
	}
	k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}
	Create(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: metav1.NamespaceDefault,
		},
		StringData: map[string]string{
			"username": Endpoint.User,
			"password": Endpoint.Pass,
		},
		Type: corev1.SecretTypeBasicAuth,
	})
	Create(t, &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: v1alpha1.Endpoint{
				Address:   net.JoinHostPort(Endpoint.Addr, "9339"),
				SecretRef: &v1alpha1.SecretReference{Name: "secret", Namespace: metav1.NamespaceDefault},
			},
		},
	})
}

// Create creates a Kubernetes object in the cluster and registers a cleanup function
// to delete it after the test completes. It fails the test if the creation fails.
func Create(t *testing.T, obj client.Object) {
	t.Helper()
	if err := k8sClient.Create(t.Context(), obj); err != nil {
		t.Fatalf("failed to create %T: %v", obj, err)
	}
	t.Logf("created %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
	t.Cleanup(func() {
		if err := k8sClient.Delete(context.Background(), obj); err != nil {
			t.Fatalf("failed to delete %T: %v", obj, err)
		}
		t.Logf("deleted %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
	})
}

// MustGetEnv retrieves the value of an environment variable
// and fails the test if it is not set.
func MustGetEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("required environment variable %q not set", key)
	}
	return value
}

// GetEnvOrDefault retrieves the value of an environment variable
// and returns a default value if it is not set.
func GetEnvOrDefault(t *testing.T, key, defaultValue string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Logf("environment variable %q not set, using default value: %s", key, defaultValue)
		return defaultValue
	}
	return value
}

// WaitTimeout returns a [script.WaitFunc] that wraps the given function
// and applies a timeout and interval for retries.
func WaitTimeout(fn script.WaitFunc, timeout, interval time.Duration) script.WaitFunc {
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(interval)
	return func(s *script.State) (stdout string, stderr string, err error) {
		defer timer.Stop()
		defer ticker.Stop()
		ctx := s.Context()
		for {
			select {
			case <-ctx.Done():
				return "", "", ctx.Err()
			case <-timer.C:
				return "", "", fmt.Errorf("timed out after %v", timeout)
			case <-ticker.C:
				stdout, stderr, err = fn(s)
				if err == nil {
					return
				}
			}
		}
	}
}

// IsObjectReady checks if a [client.Object] has a 'Ready' condition with a status of 'True'.
func IsObjectReady(obj client.Object) (bool, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return false, fmt.Errorf("failed to convert object to unstructured map: %w", err)
	}
	conditions, found, err := unstructured.NestedSlice(unstructuredMap, "status", "conditions")
	if err != nil {
		return false, fmt.Errorf("failed to access status.conditions: %w", err)
	}
	if !found {
		return false, nil
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, found, err := unstructured.NestedString(cond, "type")
		if err != nil || !found || condType != "Ready" {
			continue
		}
		condStatus, found, err := unstructured.NestedString(cond, "status")
		if err != nil || !found {
			continue
		}
		return condStatus == string(metav1.ConditionTrue), nil
	}
	return false, nil
}
