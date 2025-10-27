// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package clientutil

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ client.Reader = (*Client)(nil)

// Client is a wrapper around the controller-runtime client that allows
// to set a default namespace for all operations.
// This is useful for scenarios where resources contain references to
// other resources in the same namespace, avoiding the overhead of
// manually specifying the namespace for each operation.
type Client struct {
	r client.Reader

	// Default namespace to use for all operations
	DefaultNamespace string
}

// NewClient creates a new Client instance with the given controller-runtime reader.
func NewClient(r client.Reader, defaultNamespace string) *Client {
	return &Client{r: r, DefaultNamespace: defaultNamespace}
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server. If the key does not contain a namespace, the default
// namespace is used.
func (c *Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if key.Namespace == "" {
		key.Namespace = c.DefaultNamespace
	}

	return c.r.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the result
// returned from the Server. It will automatically restrict the request to the
// namespace that is set in the Client.
func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	opts = append(opts, client.InNamespace(c.DefaultNamespace))
	return c.r.List(ctx, list, opts...)
}

// Secret loads the referenced secret resource and returns the value of the specified key.
// If the secret does not exist or the key is not found, an error is returned.
func (c *Client) Secret(ctx context.Context, ref *v1alpha1.SecretKeySelector) ([]byte, error) {
	name := client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}

	var secret corev1.Secret
	if err := c.Get(ctx, name, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %q: %w", name.String(), err)
	}

	data, ok := secret.Data[ref.Key]
	if !ok {
		s, ok := secret.StringData[ref.Key]
		if !ok {
			return nil, fmt.Errorf("missing field %q in secret %q", ref.Key, name.String())
		}
		data = []byte(s)
	}

	return data, nil
}

// ConfigMap loads the referenced ConfigMap resource and returns the value of the specified key.
// If the ConfigMap does not exist or the key is not found, an error is returned.
func (c *Client) ConfigMap(ctx context.Context, ref *v1alpha1.ConfigMapKeySelector) ([]byte, error) {
	name := client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}

	var secret corev1.ConfigMap
	if err := c.Get(ctx, name, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %q: %w", name.String(), err)
	}

	data, ok := secret.BinaryData[ref.Key]
	if !ok {
		s, ok := secret.Data[ref.Key]
		if !ok {
			return nil, fmt.Errorf("missing field %q in configmap %q", ref.Key, name.String())
		}
		data = []byte(s)
	}

	return data, nil
}

// BasicAuth loads the username and password from the referenced secret resource.
// The secret must by of type 'kubernetes.io/basic-auth' and contain the fields 'username' and 'password'.
func (c *Client) BasicAuth(ctx context.Context, ref *v1alpha1.SecretReference) (user, pass []byte, err error) {
	name := client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}

	var secret corev1.Secret
	if err := c.Get(ctx, name, &secret); err != nil {
		return nil, nil, fmt.Errorf("failed to get secret %q: %w", name.String(), err)
	}

	if secret.Type != corev1.SecretTypeBasicAuth {
		return nil, nil, fmt.Errorf("unsupported secret type: want %q, got %q", corev1.SecretTypeBasicAuth, secret.Type)
	}

	user, ok := secret.Data[corev1.BasicAuthUsernameKey]
	if !ok || len(user) == 0 {
		return nil, nil, fmt.Errorf("missing field 'username' in secret %q", name.String())
	}

	pass, ok = secret.Data[corev1.BasicAuthPasswordKey]
	if !ok || len(pass) == 0 {
		return nil, nil, fmt.Errorf("missing field 'password' in secret %q", name.String())
	}

	return user, pass, nil
}

// Certificate loads a [tls.Certificate] from the referenced secret resource.
// The secret must by of type 'kubernetes.io/tls' and contain the fields 'tls.crt' and 'tls.key'.
func (c *Client) Certificate(ctx context.Context, ref *v1alpha1.SecretReference) (*tls.Certificate, error) {
	name := client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}

	var secret corev1.Secret
	if err := c.Get(ctx, name, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %q: %w", name.String(), err)
	}

	if secret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("unsupported secret type: want %q, got %q", corev1.SecretTypeTLS, secret.Type)
	}

	cert, ok := secret.Data[corev1.TLSCertKey]
	if !ok || len(cert) == 0 {
		return nil, fmt.Errorf("missing field 'tls.crt' in secret %q", name.String())
	}

	key, ok := secret.Data[corev1.TLSPrivateKeyKey]
	if !ok || len(key) == 0 {
		return nil, fmt.Errorf("missing field 'tls.key' in secret %q", name.String())
	}

	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	return &certificate, nil
}

// Template retrieves the template source and returns the raw template content as a byte slice.
// It supports inline templates, template references from a Secret, or template references from a ConfigMap.
func (c *Client) Template(ctx context.Context, src *v1alpha1.TemplateSource) (raw []byte, err error) {
	if src.Inline != nil {
		return []byte(*src.Inline), nil
	}

	if src.SecretRef != nil {
		data, err := c.Secret(ctx, src.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %q: %w", src.SecretRef.Name, err)
		}
		return data, nil
	}

	if src.ConfigMapRef != nil {
		data, err := c.ConfigMap(ctx, src.ConfigMapRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get configmap %q: %w", src.ConfigMapRef.Name, err)
		}
		return data, nil
	}

	return nil, errors.New("no template source specified")
}

// ListResourceVersions returns the resource versions of the given secrets.
// The returned resource versions are in the same order as the keys provided.
func (c *Client) ListResourceVersions(ctx context.Context, key ...client.ObjectKey) ([]string, error) {
	rv := make([]string, 0, len(key))
	for _, k := range key {
		var secret corev1.Secret
		if err := c.Get(ctx, k, &secret); err != nil {
			return nil, fmt.Errorf("failed to get secret %q: %w", k.String(), err)
		}
		rv = append(rv, secret.ResourceVersion)
	}
	return rv, nil
}
