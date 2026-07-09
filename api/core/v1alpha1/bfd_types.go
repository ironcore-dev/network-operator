// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BFD defines the Bidirectional Forwarding Detection configuration for interfaces, bgp peerings and static routes.
type BFD struct {
	// Enabled indicates whether BFD is enabled on the network object.
	// +required
	Enabled bool `json:"enabled"`

	// DesiredMinimumTxInterval is the minimum interval between transmission of BFD control
	// packets that the operator desires. This value is advertised to the peer.
	// The actual interval used is the maximum of this value and the remote
	// required-minimum-receive interval value.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	DesiredMinimumTxInterval *metav1.Duration `json:"desiredMinimumTxInterval,omitempty"`

	// RequiredMinimumReceive is the minimum interval between received BFD control packets
	// that this system should support. This value is advertised to the remote peer to
	// indicate the maximum frequency between BFD control packets that is acceptable
	// to the local system.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	RequiredMinimumReceive *metav1.Duration `json:"requiredMinimumReceive,omitempty"`

	// DetectionMultiplier is the number of packets that must be missed to declare
	// this session as down. The detection interval for the BFD session is calculated
	// by multiplying the value of the negotiated transmission interval by this value.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	DetectionMultiplier *int32 `json:"detectionMultiplier,omitempty"`
}
