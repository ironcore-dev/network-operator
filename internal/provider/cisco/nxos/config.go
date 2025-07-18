// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import "github.com/ironcore-dev/network-operator/api/v1alpha1"

// Parameters contains the Cisco NX-OS device specific parameters provided as ProviderConfig.
type Parameters struct {
	// DryRun indicates whether the configuration changes should be simulated without applying them to the switch.
	DryRun bool `json:"dryRun,omitempty"`
	// LogDefaultSeverity specifies the default severity level for logging.
	LogDefaultSeverity v1alpha1.Severity `json:"logDefaultSeverity,omitempty"`
	// LogHistorySeverity specifies the severity level for log history.
	LogHistorySeverity v1alpha1.Severity `json:"logHistorySeverity,omitempty"`
	// LogHistorySize specifies the size of the log history.
	LogHistorySize int `json:"logHistorySize,omitempty"`
	// LogOriginID specifies the origin ID for logging.
	LogOriginID string `json:"logOriginID,omitempty"`
	// LogSrcIf specifies the source interface to be used to reach the syslog servers.
	LogSrcIf string `json:"logSrcIf,omitempty"`
	// VlanLongName indicates whether the long-name option for VLANs should be enabled.
	VlanLongName bool `json:"vlanLongName,omitempty"`
	// CoppProfile specifies the control plane policing (CoPP) profile for the device.
	CoppProfile string `json:"coppProfile,omitempty"`
}

// DefaultParameters returns the default parameters for the Cisco NX-OS device.
func DefaultParameters() *Parameters {
	return &Parameters{
		DryRun:             false,
		LogDefaultSeverity: v1alpha1.SeverityInfo,
		LogHistorySeverity: v1alpha1.SeverityInfo,
		LogHistorySize:     1000,
		LogOriginID:        "NXOS",
		LogSrcIf:           "mgmt0",
		VlanLongName:       false,
		CoppProfile:        "strict",
	}
}
