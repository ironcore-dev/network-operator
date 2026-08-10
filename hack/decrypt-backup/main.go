// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Command decrypt-backup retrieves a remote ConfigBackup from S3 and decrypts it.
//
// Usage:
//
//	go run ./hack/decrypt-backup [-n namespace] [-o output-file] <configbackup-name>
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	controllerv1alpha1 "github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/objectstorage"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: decrypt-backup [-n namespace] [-o output-file] <configbackup-name>\n")
	flag.PrintDefaults()
}

func main() {
	namespace := flag.String("n", "default", "namespace of the ConfigBackup resource")
	output := flag.String("o", "", "output file (default: stdout)")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}
	name := flag.Arg(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	if err := run(ctx, name, *namespace, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		stop()
		os.Exit(1)
	}
	stop()
}

func run(ctx context.Context, name, namespace, output string) error {
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return fmt.Errorf("failed to register scheme: %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	k8s, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	var backup v1alpha1.ConfigBackup
	if err := k8s.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &backup); err != nil {
		return fmt.Errorf("failed to get ConfigBackup %s/%s: %w", namespace, name, err)
	}

	if backup.Spec.Type != v1alpha1.ConfigBackupTypeRemote {
		return fmt.Errorf("ConfigBackup %s is not of type Remote (got %s)", name, backup.Spec.Type)
	}
	if backup.Spec.S3 == nil {
		return fmt.Errorf("ConfigBackup %s has no S3 configuration", name)
	}
	if backup.Status.LastBackup == nil {
		return fmt.Errorf("ConfigBackup %s has no successful backup yet", name)
	}

	ref := backup.Spec.S3.CredentialsSecretRef
	if ref.Namespace == "" {
		ref.Namespace = namespace
	}

	var secret corev1.Secret
	if err := k8s.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &secret); err != nil {
		return fmt.Errorf("failed to get S3 credentials secret: %w", err)
	}

	store := objectstorage.NewClient(objectstorage.Options{
		Endpoint:        backup.Spec.S3.Endpoint,
		Region:          backup.Spec.S3.Region,
		AccessKeyID:     string(secret.Data[controllerv1alpha1.S3AccessKeyID]),
		SecretAccessKey: string(secret.Data[controllerv1alpha1.S3SecretAccessKey]),
	})

	filepath := backup.Status.LastBackup.Filepath
	prefix := fmt.Sprintf("s3://%s/", backup.Spec.S3.Bucket)
	if !strings.HasPrefix(filepath, prefix) {
		return fmt.Errorf("unexpected filepath format: %s", filepath)
	}
	key := strings.TrimPrefix(filepath, prefix)

	data, err := store.GetObject(ctx, backup.Spec.S3.Bucket, key)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", filepath, err)
	}
	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", filepath, len(data))

	if backup.Spec.S3.Encryption != nil {
		enc := backup.Spec.S3.Encryption
		ns := enc.KeySecret.Namespace
		if ns == "" {
			ns = namespace
		}
		var encSecret corev1.Secret
		if err := k8s.Get(ctx, types.NamespacedName{Name: enc.KeySecret.Name, Namespace: ns}, &encSecret); err != nil {
			return fmt.Errorf("failed to get encryption key secret: %w", err)
		}
		encKey, ok := encSecret.Data[enc.KeySecret.Key]
		if !ok {
			return fmt.Errorf("encryption key secret missing key %q", enc.KeySecret.Key)
		}

		data, err = decrypt(data, enc.Algorithm, encKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt backup: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Decrypted successfully (%d bytes plaintext)\n", len(data))
	}

	if output == "" {
		_, err = os.Stdout.Write(data)
	} else {
		err = os.WriteFile(output, data, 0o644)
		if err == nil {
			fmt.Fprintf(os.Stderr, "Written to %s\n", output)
		}
	}
	return err
}

func decrypt(data []byte, algorithm v1alpha1.EncryptionAlgorithm, key []byte) ([]byte, error) {
	var aead cipher.AEAD
	switch algorithm {
	case v1alpha1.EncryptionAES256GCM:
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		aead, err = cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
	case v1alpha1.EncryptionChaCha20Poly1305:
		var err error
		aead, err = chacha20poly1305.New(key)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	if len(data) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:aead.NonceSize()], data[aead.NonceSize():]
	return aead.Open(nil, nonce, ciphertext, nil)
}
