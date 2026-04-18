// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package tftpserver

import (
	"bytes"
	"testing"

	tftp "github.com/pin/tftp/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
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
		{name: "trim spaces", in: "  serial-test-123.boot  ", want: "test-123"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSerial(tt.in); got != tt.want {
				t.Errorf("parseSerial(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestServer(t *testing.T) {
	tests := []struct {
		name           string
		device         *v1alpha1.Device
		filename       string
		validateSource bool
		wantErr        bool
		wantContent    string
	}{
		{
			name: "serve bootscript by serial",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint:     v1alpha1.Endpoint{Address: "127.0.0.1:9339"},
					Provisioning: &v1alpha1.Provisioning{BootScript: v1alpha1.TemplateSource{Inline: new("#!/bin/sh\necho hello")}},
				},
			},
			filename:    "serial-test-serial-001.cfg",
			wantContent: "#!/bin/sh\necho hello",
		},
		{
			name: "unknown serial returns error",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint:     v1alpha1.Endpoint{Address: "127.0.0.1:9339"},
					Provisioning: &v1alpha1.Provisioning{BootScript: v1alpha1.TemplateSource{Inline: new("#!/bin/sh\necho hello")}},
				},
			},
			filename: "serial-unknown.cfg",
			wantErr:  true,
		},
		{
			name: "empty bootscript returns error",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-script-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "127.0.0.1:9339"},
				},
			},
			filename: "serial-test-serial-001.cfg",
			wantErr:  true,
		},
		{
			name: "verify mode: matching IP and serial passes",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint:     v1alpha1.Endpoint{Address: "127.0.0.1:9339"},
					Provisioning: &v1alpha1.Provisioning{BootScript: v1alpha1.TemplateSource{Inline: new("#!/bin/sh\necho hello")}},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "test-serial-001"},
			},
			filename:       "serial-test-serial-001.cfg",
			validateSource: true,
			wantContent:    "#!/bin/sh\necho hello",
		},
		{
			name: "verify mode: IP mismatch rejected",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint:     v1alpha1.Endpoint{Address: "10.0.0.99:9339"},
					Provisioning: &v1alpha1.Provisioning{BootScript: v1alpha1.TemplateSource{Inline: new("#!/bin/sh\necho hello")}},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "test-serial-001"},
			},
			filename:       "serial-test-serial-001.cfg",
			validateSource: true,
			wantErr:        true,
		},
		{
			name: "verify mode: serial mismatch rejected",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "test-serial-001"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint:     v1alpha1.Endpoint{Address: "127.0.0.1:9339"},
					Provisioning: &v1alpha1.Provisioning{BootScript: v1alpha1.TemplateSource{Inline: new("#!/bin/sh\necho hello")}},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "test-serial-001"},
			},
			filename:       "serial-different-serial.cfg",
			validateSource: true,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tt.device).
				WithIndex(&v1alpha1.Device{}, deviceutil.DeviceEndpointIPField, func(o client.Object) []string {
					return []string{o.(*v1alpha1.Device).EndpointIP()}
				}).
				Build()

			srv := &Server{
				Client:         fc,
				Logger:         klog.NewKlogr(),
				ValidateSource: tt.validateSource,
				Port:           16900,
			}
			go func() {
				if err := srv.Start(t.Context()); err != nil {
					t.Errorf("start server: %v", err)
				}
			}()

			tc, err := tftp.NewClient("127.0.0.1:16900")
			if err != nil {
				t.Fatalf("create TFTP client: %v", err)
			}

			wt, err := tc.Receive(tt.filename, "octet")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var buf bytes.Buffer
			if _, err = wt.WriteTo(&buf); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}
			if got := buf.String(); got != tt.wantContent {
				t.Errorf("content = %q, want %q", got, tt.wantContent)
			}
		})
	}
}

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}
