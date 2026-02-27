// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package providerconfig

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// Scope wraps objects fetched from k8s and validated by registered provider config validators.
// This allows the provider to remain independent of Kubernetes client libraries.
type Scope struct {
	obj *unstructured.Unstructured
}

// NewScope creates a new Scope from an unstructured object.
func NewScope(obj *unstructured.Unstructured) *Scope {
	return &Scope{obj: obj}
}

// Into converts the underlying unstructured object into the specified type.
func (s *Scope) Into(v any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(s.obj.Object, v)
}

// Raw returns the underlying unstructured object.
func (s *Scope) Raw() *unstructured.Unstructured {
	return s.obj
}

// Validator validates a provider-specific configuration object referenced by a core resource.
// It receives a client.Reader so that callers can choose whether to use the cached client or
// the uncached APIReader.
type Validator func(ctx context.Context, r client.Reader, parent client.Object, ref *corev1.TypedLocalObjectReference) (*Scope, error)

var (
	mu     sync.RWMutex
	lookup = map[schema.GroupVersionKind]Validator{}
)

// RegisterValidator registers a provider config validator for the given GVK.
// Registration should be called from init() in provider config packages.
func RegisterValidator(gvk schema.GroupVersionKind, fn Validator) {
	mu.Lock()
	defer mu.Unlock()
	lookup[gvk] = fn
}

// GetValidator returns the registered validator for the GVK, if any.
func GetValidator(gvk schema.GroupVersionKind) (Validator, bool) {
	mu.RLock()
	defer mu.RUnlock()
	fn, ok := lookup[gvk]
	return fn, ok
}
