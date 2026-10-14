// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestMaintenanceCommands(t *testing.T) {
	root := NewCmdNet(genericiooptions.IOStreams{}, "test")
	actions := []string{
		v1alpha1.DeviceMaintenanceReboot,
		v1alpha1.DeviceMaintenanceFactoryReset,
		v1alpha1.DeviceMaintenanceReprovision,
		v1alpha1.DeviceMaintenanceResetPhase,
		v1alpha1.DeviceMaintenanceSkipProvisioning,
		v1alpha1.DeviceMaintenanceFirmwareUpgrade,
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			command, _, err := root.Find([]string{action})
			if err != nil {
				t.Fatal(err)
			}
			if command.Name() != action {
				t.Fatalf("command name = %q, want %q", command.Name(), action)
			}
			if got := command.Flags().Lookup("yes") != nil; got != maintenanceActionRequiresConfirmation(action) {
				t.Fatalf("--yes flag present = %t, want %t", got, maintenanceActionRequiresConfirmation(action))
			}
			if err := command.Args(command, nil); err == nil {
				t.Fatal("command accepted no device name")
			}
			if err := command.Args(command, []string{"leaf1", "leaf2"}); err == nil {
				t.Fatal("command accepted multiple device names")
			}
		})
	}
}

func TestMaintenanceCommandRequiresFirmwareURL(t *testing.T) {
	streams := genericiooptions.IOStreams{Out: new(bytes.Buffer), ErrOut: new(bytes.Buffer)}
	root := NewCmdNet(streams, "test")
	root.SetArgs([]string{v1alpha1.DeviceMaintenanceFirmwareUpgrade, "leaf1"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "url" not set`) {
		t.Fatalf("Execute() error = %v, want missing url flag error", err)
	}
}

func TestMaintenanceConfirmation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "lowercase yes", input: "y\n"},
		{name: "uppercase yes", input: "Y\n"},
		{name: "no", input: "n\n", wantErr: true},
		{name: "empty", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := confirmMaintenanceOperation(strings.NewReader(test.input), new(bytes.Buffer), v1alpha1.DeviceMaintenanceReboot, "leaf1")
			if (err != nil) != test.wantErr {
				t.Fatalf("confirmMaintenanceOperation() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestMaintenancePatch(t *testing.T) {
	tests := []struct {
		name string
		opts MaintenanceOptions
		want map[string]any
	}{
		{
			name: "plain action",
			opts: MaintenanceOptions{Action: v1alpha1.DeviceMaintenanceReboot, ResourceVersion: "100"},
			want: map[string]any{"metadata": map[string]any{
				"resourceVersion": "100",
				"annotations": map[string]any{
					v1alpha1.DeviceMaintenanceAnnotation: v1alpha1.DeviceMaintenanceReboot,
				},
			}},
		},
		{
			name: "firmware upgrade with checksum",
			opts: MaintenanceOptions{
				Action:          v1alpha1.DeviceMaintenanceFirmwareUpgrade,
				FirmwareURL:     "https://example.com/nxos.bin",
				FirmwareMD5:     "0123456789abcdef",
				ResourceVersion: "200",
			},
			want: map[string]any{"metadata": map[string]any{
				"resourceVersion": "200",
				"annotations": map[string]any{
					v1alpha1.DeviceMaintenanceAnnotation:               v1alpha1.DeviceMaintenanceFirmwareUpgrade,
					v1alpha1.DeviceMaintenanceFirmwareTargetAnnotation: `{"url":"https://example.com/nxos.bin","md5":"0123456789abcdef"}`,
				},
			}},
		},
		{
			name: "firmware upgrade without checksum omits md5",
			opts: MaintenanceOptions{
				Action:          v1alpha1.DeviceMaintenanceFirmwareUpgrade,
				FirmwareURL:     "https://example.com/nxos.bin",
				ResourceVersion: "300",
			},
			want: map[string]any{"metadata": map[string]any{
				"resourceVersion": "300",
				"annotations": map[string]any{
					v1alpha1.DeviceMaintenanceAnnotation:               v1alpha1.DeviceMaintenanceFirmwareUpgrade,
					v1alpha1.DeviceMaintenanceFirmwareTargetAnnotation: `{"url":"https://example.com/nxos.bin"}`,
				},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			patch, err := test.opts.buildPatch()
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(patch, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("patch = %#v, want %#v", got, test.want)
			}
		})
	}
}
