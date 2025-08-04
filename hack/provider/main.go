// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	// Import all supported provider implementations.
	_ "github.com/ironcore-dev/network-operator/internal/provider/openconfig"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var (
	address      = flag.String("address", "", "API endpoint address (required)")
	username     = flag.String("username", "", "Username for authentication (required)")
	password     = flag.String("password", "", "Password for authentication (required)")
	file         = flag.String("file", "", "Path to Kubernetes resource manifest file (required)")
	providerName = flag.String("provider", "openconfig", "Provider implementation to use")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] <create|delete>\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "A debug tool for testing provider implementations.\n\n")
	fmt.Fprintf(os.Stderr, "Arguments:\n")
	fmt.Fprintf(os.Stderr, "  create|delete    Operation to perform on the resource\n\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample:\n")
	fmt.Fprintf(os.Stderr, "  %s -address=192.168.1.1:9339 -username=admin -password=secret -file=config/samples/v1alpha1_device.yaml create\n", os.Args[0])
}

func validateFlags() error {
	if *address == "" {
		return errors.New("address flag is required")
	}
	if *username == "" {
		return errors.New("username flag is required")
	}
	if *password == "" {
		return errors.New("password flag is required")
	}
	if *file == "" {
		return errors.New("file flag is required")
	}
	return nil
}

func validatePositionalArgs() (string, error) {
	if len(flag.Args()) != 1 {
		return "", errors.New("exactly one positional argument (create|delete) is required")
	}

	operation := flag.Args()[0]
	if operation != "create" && operation != "delete" {
		return "", fmt.Errorf("positional argument must be either 'create' or 'delete', got: %s", operation)
	}

	return operation, nil
}

func loadAndUnmarshalResource(filePath string) (runtime.Object, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	if err = v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return nil, fmt.Errorf("failed to add scheme: %w", err)
	}

	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

	json, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	obj, _, err := decoder.Decode(json, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resource: %w", err)
	}

	return obj, nil
}

func printResourceInfo(obj runtime.Object) {
	switch resource := obj.(type) {
	case *v1alpha1.Interface:
		fmt.Printf("Loaded Interface: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Interface Name: %s\n", resource.Spec.Name)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
	default:
		fmt.Printf("Loaded resource of unknown type: %T\n", resource)
	}
}

func main() {
	flag.Usage = usage

	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			flag.Usage()
			os.Exit(0)
		}
	}

	flag.Parse()

	if err := validateFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	operation, err := validatePositionalArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	resource, err := loadAndUnmarshalResource(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading resource: %v\n", err)
		os.Exit(1)
	}

	obj, ok := resource.(client.Object)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: resource is not a client.Object\n")
		os.Exit(1)
	}
	if obj.GetNamespace() == "" {
		obj.SetNamespace(metav1.NamespaceDefault)
	}

	fmt.Printf("=== Debug Tool Configuration ===\n")
	fmt.Printf("Address: %s\n", *address)
	fmt.Printf("Username: %s\n", *username)
	fmt.Printf("Password: %s\n", "[REDACTED]")
	fmt.Printf("Resource File: %s\n", *file)
	fmt.Printf("Provider: %s\n", *providerName)
	fmt.Printf("Operation: %s\n", operation)
	fmt.Printf("\n=== Resource Information ===\n")
	printResourceInfo(resource)

	prov, err := provider.Get(*providerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting provider: %v\n", err)
		os.Exit(1)
	}

	device := &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: obj.GetNamespace(),
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: &v1alpha1.Endpoint{
				Address: *address,
				SecretRef: &corev1.SecretReference{
					Name:      "test-secret",
					Namespace: obj.GetNamespace(),
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: obj.GetNamespace(),
		},
		Data: map[string][]byte{
			"username": []byte(*username),
			"password": []byte(*password),
		},
		Type: corev1.SecretTypeBasicAuth,
	}

	obj.SetLabels(map[string]string{v1alpha1.DeviceLabel: device.Name})

	c := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(device, secret).
		Build()

	ctx := clientutil.IntoContext(context.Background(), c, obj.GetNamespace())

	fmt.Printf("\n=== Operation Status ===\n")
	switch operation {
	case "create":
		switch resource := obj.(type) {
		case *v1alpha1.Interface:
			err = prov.CreateInterface(ctx, resource)
		default:
			fmt.Printf("Loaded resource of unknown type: %T\n", resource)
		}
	case "delete":
		switch resource := obj.(type) {
		case *v1alpha1.Interface:
			err = prov.DeleteInterface(ctx, resource)
		default:
			fmt.Printf("Loaded resource of unknown type: %T\n", resource)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error performing operation: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Provider tool completed successfully.\n")
}
