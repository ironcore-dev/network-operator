// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// LabelFlags holds label-based filter flags used by get and pause commands.
type LabelFlags struct {
	Device string

	Aggregate string
	VRF       string

	RoutedVLAN string
	EVI        string
}

// AddCommonFlags registers the --device flag available for all resource types.
func (l *LabelFlags) AddCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&l.Device, "device", "d", "", fmt.Sprintf("Filter by %s label", v1alpha1.DeviceLabel))
}

// AddInterfaceFlags registers the --aggregate and --vrf flags for Interface resources.
func (l *LabelFlags) AddInterfaceFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&l.Aggregate, "aggregate", "", fmt.Sprintf("Filter by %s label", v1alpha1.AggregateLabel))
	cmd.Flags().StringVar(&l.VRF, "vrf", "", fmt.Sprintf("Filter by %s label", v1alpha1.VRFLabel))
}

// AddVLANFlags registers the --routed-vlan and --evi flags for VLAN resources.
func (l *LabelFlags) AddVLANFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&l.RoutedVLAN, "routed-vlan", "", fmt.Sprintf("Filter by %s label", v1alpha1.RoutedVLANLabel))
	cmd.Flags().StringVar(&l.EVI, "evi", "", fmt.Sprintf("Filter by %s label", v1alpha1.L2VNILabel))
}

// BuildSelector returns a comma-separated Kubernetes label selector string
// from the populated flag values.
func (l *LabelFlags) BuildSelector() string {
	parts := []string{}

	if l.Device != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", v1alpha1.DeviceLabel, l.Device))
	}
	if l.Aggregate != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", v1alpha1.AggregateLabel, l.Aggregate))
	}
	if l.VRF != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", v1alpha1.VRFLabel, l.VRF))
	}
	if l.RoutedVLAN != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", v1alpha1.RoutedVLANLabel, l.RoutedVLAN))
	}
	if l.EVI != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", v1alpha1.L2VNILabel, l.EVI))
	}

	return strings.Join(parts, ",")
}
