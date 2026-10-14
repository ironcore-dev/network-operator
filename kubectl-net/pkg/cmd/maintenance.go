// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// MaintenanceOptions holds the configuration for a Device maintenance command.
type MaintenanceOptions struct {
	Root       *RootOptions
	Action     string
	PrintFlags *genericclioptions.PrintFlags

	FirmwareURL     string
	FirmwareMD5     string
	Yes             bool
	ResourceVersion string
	Namespace       string
}

// NewCmdMaintenance returns the Device maintenance commands.
func NewCmdMaintenance(root *RootOptions) []*cobra.Command {
	actions := []struct {
		name  string
		short string
	}{
		{v1alpha1.DeviceMaintenanceReboot, "Reboot a Device"},
		{v1alpha1.DeviceMaintenanceFactoryReset, "Factory reset a Device"},
		{v1alpha1.DeviceMaintenanceReprovision, "Reprovision a Device"},
		{v1alpha1.DeviceMaintenanceResetPhase, "Reset a Device phase"},
		{v1alpha1.DeviceMaintenanceSkipProvisioning, "Skip provisioning a Device"},
		{v1alpha1.DeviceMaintenanceFirmwareUpgrade, "Upgrade Device firmware"},
	}

	commands := make([]*cobra.Command, 0, len(actions))
	for _, action := range actions {
		o := &MaintenanceOptions{
			Root:       root,
			Action:     action.name,
			PrintFlags: genericclioptions.NewPrintFlags(action.name),
		}
		cmd := &cobra.Command{
			Use:   action.name + " DEVICE",
			Short: action.short,
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				if err := o.Complete(); err != nil {
					return err
				}
				return o.Run(args[0])
			},
		}
		if maintenanceActionRequiresConfirmation(action.name) {
			cmd.Flags().BoolVarP(&o.Yes, "yes", "y", false, "Skip confirmation")
		}
		if action.name == v1alpha1.DeviceMaintenanceFirmwareUpgrade {
			cmd.Flags().StringVar(&o.FirmwareURL, "url", "", "Firmware image URL")
			cmd.Flags().StringVar(&o.FirmwareMD5, "md5", "", "Firmware image MD5 checksum")
			_ = cmd.MarkFlagRequired("url")
		}
		o.PrintFlags.AddFlags(cmd)
		commands = append(commands, cmd)
	}
	return commands
}

// Complete resolves the target namespace from the kubeconfig.
func (o *MaintenanceOptions) Complete() error {
	if o.Root.AllNamespaces {
		return errors.New("a maintenance operation cannot be performed across all namespaces")
	}
	namespace, _, err := o.Root.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// Run annotates a Device to request a maintenance operation.
func (o *MaintenanceOptions) Run(name string) error {
	resourceName := "devices." + v1alpha1.GroupVersion.Group
	result, err := buildResourceResult(o.Root.ConfigFlags, o.Namespace, false, resourceName, []string{name}, "")
	if err != nil {
		return err
	}
	infos, err := result.Infos()
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return fmt.Errorf("expected one Device, found %d", len(infos))
	}
	device := infos[0].Object.(*unstructured.Unstructured)
	if _, ok := device.GetAnnotations()[v1alpha1.DeviceMaintenanceAnnotation]; ok {
		return fmt.Errorf("device %q already has a maintenance operation", name)
	}
	o.ResourceVersion = device.GetResourceVersion()
	if err := o.confirm(name); err != nil {
		return err
	}

	restConfig, err := o.Root.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	patch, err := o.buildPatch()
	if err != nil {
		return err
	}
	obj, err := dyn.Resource(infos[0].Mapping.Resource).Namespace(infos[0].Namespace).Patch(context.Background(), name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	return printer.PrintObj(obj, o.Root.IOStreams.Out)
}

func (o *MaintenanceOptions) confirm(name string) error {
	if o.Yes || !maintenanceActionRequiresConfirmation(o.Action) {
		return nil
	}

	input, ok := o.Root.IOStreams.In.(*os.File)
	if !ok {
		return errors.New("maintenance confirmation requires a terminal; use --yes to skip confirmation")
	}
	info, err := input.Stat()
	if err != nil {
		return fmt.Errorf("failed to inspect maintenance confirmation input: %w", err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("maintenance confirmation requires a terminal; use --yes to skip confirmation")
	}
	return confirmMaintenanceOperation(input, o.Root.IOStreams.Out, o.Action, name)
}

func maintenanceActionRequiresConfirmation(action string) bool {
	switch action {
	case v1alpha1.DeviceMaintenanceReboot,
		v1alpha1.DeviceMaintenanceFactoryReset,
		v1alpha1.DeviceMaintenanceReprovision,
		v1alpha1.DeviceMaintenanceFirmwareUpgrade:
		return true
	default:
		return false
	}
}

func confirmMaintenanceOperation(in io.Reader, out io.Writer, action, name string) error {
	description := strings.ReplaceAll(action, "-", " ")
	description = strings.ToUpper(description[:1]) + description[1:]
	fmt.Fprintf(out, "%s Device %q? [y/N]: ", description, name)

	response, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to read maintenance confirmation: %w", err)
	}
	if strings.TrimSpace(response) != "y" && strings.TrimSpace(response) != "Y" {
		return errors.New("maintenance operation cancelled")
	}
	return nil
}

// buildPatch returns the JSON merge-patch payload for the maintenance annotations.
func (o *MaintenanceOptions) buildPatch() ([]byte, error) {
	annotations := map[string]any{v1alpha1.DeviceMaintenanceAnnotation: o.Action}
	if o.Action == v1alpha1.DeviceMaintenanceFirmwareUpgrade {
		target, err := json.Marshal(struct {
			URL string `json:"url"`
			MD5 string `json:"md5,omitempty"`
		}{o.FirmwareURL, o.FirmwareMD5})
		if err != nil {
			return nil, err
		}
		annotations[v1alpha1.DeviceMaintenanceFirmwareTargetAnnotation] = string(target)
	}
	return json.Marshal(map[string]any{
		"metadata": map[string]any{
			"resourceVersion": o.ResourceVersion,
			"annotations":     annotations,
		},
	})
}
