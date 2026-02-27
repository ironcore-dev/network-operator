// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package providerconfig

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestRegisterAndGetValidator(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "test.example.com", Version: "v1", Kind: "TestConfig"}

	called := false
	testValidator := func(ctx context.Context, c client.Reader, parent client.Object, ref *corev1.TypedLocalObjectReference) (*Scope, error) {
		called = true
		return nil, nil
	}

	RegisterValidator(gvk, testValidator)

	fn, ok := GetValidator(gvk)
	assert.True(t, ok, "validator should be found")
	assert.NotNil(t, fn)

	_, err := fn(context.Background(), nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, called, "validator function should have been called")
}

func TestGetValidator_NotFound(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "unknown.example.com", Version: "v1", Kind: "Unknown"}

	fn, ok := GetValidator(gvk)
	assert.False(t, ok)
	assert.Nil(t, fn)
}
