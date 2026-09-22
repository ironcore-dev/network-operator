// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package paused

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

func TestEnsureCondition(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	device := &v1alpha1.Device{
		Name: "device",
		Spec: v1alpha1.DeviceSpec{
			Paused: true,
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(device).
		WithObjects(device).
		Build()

	paused, err := EnsureCondition(ctx, c, device, device)
	if err != nil {
		t.Fatalf("EnsureCondition() error = %v", err)
	}
	if !paused {
		t.Fatal("EnsureCondition() paused = false, want true")
	}

	stored := new(v1alpha1.Device)
	if err := c.Get(ctx, client.ObjectKeyFromObject(device), stored); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	condition := conditions.Get(stored, v1alpha1.PausedCondition)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("paused condition = %#v, want true", condition)
	}
}
