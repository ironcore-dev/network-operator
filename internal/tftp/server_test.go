// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package tftpserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestParseSerial(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "strip serial prefix and extension", in: "serial-test-123.boot", want: "test-123"},
		{name: "no prefix with extension", in: "test-123.boot", want: "test-123"},
		{name: "keep first segment before dot", in: "serial-test-123.boot.cfg", want: "test-123"},
		{name: "trim spaces", in: "  serial-test-123.boot  ", want: "serial-test-123"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSerial(tt.in))
		})
	}
}

func TestEndpointIP(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ip and port", in: "10.0.0.8:22", want: "10.0.0.8"},
		{name: "ip only", in: "10.0.0.8", want: "10.0.0.8"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, endpointIP(tt.in))
		})
	}
}

func TestLookupBySerial(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(sch))
	require.NoError(t, corev1.AddToScheme(sch))

	dev := &corev1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-1",
			Namespace: "default",
			Labels: map[string]string{
				corev1.DeviceSerialLabel: "SER-001",
			},
		},
		Spec: corev1.DeviceSpec{Endpoint: corev1.Endpoint{Address: "10.0.0.10:22"}},
	}

	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(dev).Build()
	srv := &Server{reader: cl}

	t.Run("found", func(t *testing.T) {
		got, err := srv.lookupBySerial(context.Background(), "SER-001")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "device-1", got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		got, err := srv.lookupBySerial(context.Background(), "SER-404")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestLookupByIP(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(sch))
	require.NoError(t, corev1.AddToScheme(sch))

	dev := &corev1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-1",
			Namespace: "default",
		},
		Spec: corev1.DeviceSpec{Endpoint: corev1.Endpoint{Address: "10.0.0.10:22"}},
	}

	cl := fake.NewClientBuilder().
		WithScheme(sch).
		WithObjects(dev).
		WithIndex(&corev1.Device{}, deviceEndpointIPField, func(obj client.Object) []string {
			d, ok := obj.(*corev1.Device)
			if !ok {
				return nil
			}
			return []string{endpointIP(d.Spec.Endpoint.Address)}
		}).
		Build()

	srv := &Server{reader: cl}

	t.Run("found", func(t *testing.T) {
		got, err := srv.lookupByIP(context.Background(), "10.0.0.10")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "device-1", got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		got, err := srv.lookupByIP(context.Background(), "10.0.0.99")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestResolveBootScript(t *testing.T) {
	sch := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(sch))
	require.NoError(t, corev1.AddToScheme(sch))

	cl := fake.NewClientBuilder().WithScheme(sch).Build()
	ctx := context.Background()

	t.Run("nil provisioning returns nil", func(t *testing.T) {
		got := resolveBootScript(ctx, cl, "default", nil)
		assert.Nil(t, got)
	})

	t.Run("inline bootscript", func(t *testing.T) {
		inline := "#!/bin/sh\necho hello"
		p := &corev1.Provisioning{BootScript: corev1.TemplateSource{Inline: &inline}}

		got := resolveBootScript(ctx, cl, "default", p)
		require.NotNil(t, got)
		assert.Equal(t, inline, string(got))
	})

	t.Run("missing template source returns nil", func(t *testing.T) {
		p := &corev1.Provisioning{BootScript: corev1.TemplateSource{}}

		got := resolveBootScript(ctx, cl, "default", p)
		assert.Nil(t, got)
	})
}
